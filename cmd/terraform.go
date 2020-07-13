package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/crdinfo"
	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type terraformCmd struct {
	Workspace      string
	Namespace      string
	Path           string
	Args           []string
	Command        string
	TimeoutClient  time.Duration
	TimeoutPod     time.Duration
	TimeoutQueue   time.Duration
	KubeConfigPath string

	cmd *cobra.Command

	scheme *runtime.Scheme
}

func newTerraformCmds() []*cobra.Command {
	var cmds []*cobra.Command

	for _, c := range v1alpha1.Commands {
		cc := &terraformCmd{}
		cc.Command = c
		cc.cmd = &cobra.Command{
			Use:   c,
			Short: fmt.Sprintf("Run %s", c),
			RunE:  cc.doTerraformCmd,
		}
		cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "terraform config path")
		cc.cmd.Flags().DurationVar(&cc.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
		cc.cmd.Flags().DurationVar(&cc.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
		cc.cmd.Flags().DurationVar(&cc.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
		cc.cmd.Flags().StringVar(&cc.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")
		cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "", "Kubernetes namespace of workspace (defaults to namespace set in .terraform/environment, or if that does not exist, \"default\")")

		cc.cmd.Flags().StringVar(&cc.Workspace, "workspace", "", "Workspace name (defaults to workspace set in .terraform/environment or, if that does not exist, \"default\")")

		cmds = append(cmds, cc.cmd)
	}

	return cmds
}

func (t *terraformCmd) doTerraformCmd(cmd *cobra.Command, args []string) error {
	if err := unmarshalV(t); err != nil {
		return err
	}

	if t.Command == "shell" {
		t.Args = shellWrapArgs(args)
	} else {
		t.Args = args
	}

	// Workspace config precedence:
	// 1. Flag
	// 2. Environment File
	// 3. "default"
	if t.Workspace == "" {
		_, workspace, err := readEnvironmentFile(t.Path)
		if errors.IsNotFound(err) {
			t.Workspace = "default"
		} else if err != nil {
			return err
		}
		t.Workspace = workspace
	}

	// Namespace config precedence:
	// 1. Flag
	// 2. Environment File
	// 3. "default"
	if t.Namespace == "" {
		namespace, _, err := readEnvironmentFile(t.Path)
		if errors.IsNotFound(err) {
			t.Namespace = "default"
		} else if err != nil {
			return err
		}
		t.Namespace = namespace
	}

	// Get built-in scheme
	t.scheme = scheme.Scheme
	// And add our CRDs
	apis.AddToScheme(t.scheme)

	return t.run()
}

// create dynamic client
// exit if workspace is unhealthy
// construct command resource
// create tarball
// embed tarball in configmap
// deploy resources
// watch queue until we are front of queue
// get logs
// get pod logs stream
// attach to pod (falling back to logs on error)
func (t *terraformCmd) run() error {
	crd, ok := crdinfo.Inventory[t.Command]
	if !ok {
		return fmt.Errorf("Could not find Custom Resource Definition '%s'", t.Command)
	}

	config, err := configFromPath(t.KubeConfigPath)
	if err != nil {
		return err
	}

	// Controller-runtime client for constructing command resource
	rc, err := client.New(config, client.Options{Scheme: t.scheme})
	if err != nil {
		return err
	}

	// client-go client for creating configmap resource
	kc, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// Check namespace exists
	_, err = kc.CoreV1().Namespaces().Get(t.Namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{"namespace": t.Namespace}).Debug("resource checked")

	// Check workspace resource exists and is healthy
	ws := v1alpha1.Workspace{}
	wsNamespacedName := types.NamespacedName{Name: t.Workspace, Namespace: t.Namespace}
	if err := rc.Get(context.TODO(), wsNamespacedName, &ws); err != nil {
		return err
	}
	log.WithFields(log.Fields{"workspace": t.Workspace, "namespace": t.Namespace}).Debug("resource checked")

	wsHealth := ws.Status.Conditions.GetCondition(v1alpha1.ConditionHealthy)
	if wsHealth == nil {
		return fmt.Errorf("Workspace %s is missing a WorkspaceHealthy condition", t.Workspace)
	}
	if wsHealth.Status != corev1.ConditionTrue {
		return fmt.Errorf("Workspace %s is unhealthy; %s", t.Workspace, wsHealth.Message)
	}

	// Construct and deploy command resource
	cmdRes, err := t.createCommand(rc, crd)
	if err != nil {
		return err
	}

	// Delete command resource upon program termination
	defer func() {
		rc.Delete(context.TODO(), cmdRes)
	}()

	// Compile tarball of terraform module
	tarball, err := t.createTar()
	if err != nil {
		return err
	}

	// Construct and deploy ConfigMap resource
	_, err = t.createConfigMap(kc, cmdRes, tarball)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"pod":       cmdRes.GetName(),
		"namespace": t.Namespace,
	}).Debug("awaiting readiness")
	pod, err := t.waitUntilPodRunningAndReady(kc, &corev1.Pod{}, cmdRes.GetName(), t.Namespace)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"pod":       cmdRes.GetName(),
		"namespace": t.Namespace,
	}).Debug("retrieve log stream")
	req := kc.CoreV1().Pods(t.Namespace).GetLogs(cmdRes.GetName(), &corev1.PodLogOptions{Follow: true})
	logs, err := req.Stream()
	if err != nil {
		return err
	}
	defer logs.Close()

	// Attach to pod tty
	done := make(chan error)
	go func() {
		log.WithFields(log.Fields{
			"pod": cmdRes.GetName(),
		}).Debug("attaching")

		drawDivider()

		err := t.handleAttachPod(*config, pod)
		if err != nil {
			// TODO: use log fields
			log.Warn("Failed to attach to pod TTY; falling back to streaming logs")
			_, err = io.Copy(os.Stdout, logs)
			done <- err
		} else {
			done <- nil
		}
	}()

	// Let operator know we're now streaming logs
	retry.RetryOnConflict(retry.DefaultRetry, func() error {
		objKey := types.NamespacedName{Name: cmdRes.GetName(), Namespace: t.Namespace}
		err = rc.Get(context.TODO(), objKey, cmdRes)
		if err != nil {
			done <- err
		} else {
			annotations := cmdRes.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations["stok.goalspike.com/client"] = "Ready"
			cmdRes.SetAnnotations(annotations)

			return rc.Update(context.TODO(), cmdRes, &client.UpdateOptions{})
		}
		return nil
	})

	return <-done
}

func (t *terraformCmd) waitUntilPodRunningAndReady(client kubernetes.Interface, pod *corev1.Pod, name, namespace string) (*corev1.Pod, error) {
	obj, err := waitUntil(client.CoreV1().RESTClient(), pod, name, namespace, "pods", podRunningAndReady, t.TimeoutPod)
	return obj.(*corev1.Pod), err
}

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// podRunningAndReady returns true if the pod is running and ready, false if the pod has not
// yet reached those states, returns ErrPodCompleted if the pod has run to completion, or
// an error in any other case.
func podRunningAndReady(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}
	switch t := event.Object.(type) {
	case *corev1.Pod:
		switch t.Status.Phase {
		case corev1.PodSucceeded:
			return false, ErrPodCompleted
		case corev1.PodFailed:
			return false, fmt.Errorf(t.Status.ContainerStatuses[0].State.Terminated.Message)
		case corev1.PodRunning:
			conditions := t.Status.Conditions
			if conditions == nil {
				return false, nil
			}
			for i := range conditions {
				if conditions[i].Type == corev1.PodReady &&
					conditions[i].Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
