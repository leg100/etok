package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type terraformCmd struct {
	Workspace     string
	Namespace     string
	Context       string
	Path          string
	Args          []string
	Kind          string
	TimeoutClient time.Duration
	TimeoutPod    time.Duration
	TimeoutQueue  time.Duration

	cmd *cobra.Command

	debug   bool
	factory k8s.FactoryInterface
}

func newTerraformCmds(f k8s.FactoryInterface) []*cobra.Command {
	var cmds []*cobra.Command

	for _, kind := range command.CommandKinds {
		cc := &terraformCmd{}
		cc.Kind = kind
		cc.cmd = &cobra.Command{
			Use:   command.CommandKindToCLI(kind),
			Short: fmt.Sprintf("Run %s", command.CommandKindToCLI(kind)),
			RunE:  cc.doTerraformCmd,
		}
		cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "terraform config path")
		cc.cmd.Flags().DurationVar(&cc.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
		cc.cmd.Flags().DurationVar(&cc.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
		cc.cmd.Flags().DurationVar(&cc.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
		cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "", "Kubernetes namespace of workspace (defaults to namespace set in .terraform/environment, or \"default\")")

		cc.cmd.Flags().StringVar(&cc.Workspace, "workspace", "", "Workspace name (defaults to workspace set in .terraform/environment or, \"default\")")
		cc.cmd.Flags().StringVar(&cc.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

		// Add flags registered by imported packages (controller-runtime)
		cc.cmd.Flags().AddGoFlagSet(flag.CommandLine)

		cc.factory = f

		cmds = append(cmds, cc.cmd)
	}

	return cmds
}

func (t *terraformCmd) doTerraformCmd(cmd *cobra.Command, args []string) error {
	debug, err := cmd.InheritedFlags().GetBool("debug")
	if err != nil {
		return err
	}
	t.debug = debug

	// TODO: remove
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Args = args

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
	// Get client from factory. Embeds controller-runtime client
	rc, err := t.factory.NewClient(scheme.Scheme, t.Context)
	if err != nil {
		return err
	}

	// Check namespace exists
	if err := rc.Get(context.TODO(), types.NamespacedName{Name: t.Namespace}, &corev1.Namespace{}); err != nil {
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

	// Generate unique name shared by command and configmap resources (and command ctrl will spawn a
	// pod with this name, too)
	name := command.GenerateName(t.Kind)

	// Construct and deploy command resource
	cmdRes, err := t.createCommand(rc, name, name)
	if err != nil {
		return err
	}

	// Delete command resource upon program termination. This will take care of deleting the
	// configmap below too because the configmap is 'owned' by the command resource and k8s will
	// therefore delete it automatically.
	defer rc.Delete(context.TODO(), cmdRes)

	// Compile tarball of terraform module
	tarball, err := t.createTar()
	if err != nil {
		return err
	}

	if len(tarball) > v1alpha1.MaxConfigSize {
		return fmt.Errorf("max config size exceeded; current=%d; max=%d", len(tarball), v1alpha1.MaxConfigSize)
	}

	// Construct and deploy ConfigMap resource
	// TODO: perform this task in parallel to constructing and deploying command resource above to
	// increase performance
	_, err = t.createConfigMap(rc, cmdRes, tarball, name, v1alpha1.CommandDefaultConfigMapKey)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"pod":       name,
		"namespace": t.Namespace,
	}).Debug("awaiting readiness")
	pod, err := waitUntilPodRunningAndReady(rc, &corev1.Pod{}, t.Namespace, name, t.TimeoutPod)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"pod":       name,
		"namespace": t.Namespace,
	}).Debug("retrieve log stream")
	logs, err := rc.GetLogs(t.Namespace, name, &corev1.PodLogOptions{Follow: true})
	if err != nil {
		return err
	}
	defer logs.Close()

	// Attach to pod tty
	done := make(chan error)
	go func() {
		log.WithFields(log.Fields{
			"pod": name,
		}).Debug("attaching")

		drawDivider()

		err := rc.Attach(pod)
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
		objKey := types.NamespacedName{Name: name, Namespace: t.Namespace}
		err = rc.Get(context.TODO(), objKey, cmdRes)
		if err != nil {
			done <- err
		} else {
			// Delete annotation WaitAnnotationKey, giving the runner the signal to start
			annotations := cmdRes.GetAnnotations()
			delete(annotations, v1alpha1.WaitAnnotationKey)
			cmdRes.SetAnnotations(annotations)

			return rc.Update(context.TODO(), cmdRes, &client.UpdateOptions{})
		}
		return nil
	})

	return <-done
}

func waitUntilPodRunningAndReady(rc k8s.Client, pod *corev1.Pod, namespace, name string, timeout time.Duration) (*corev1.Pod, error) {
	err := wait.PollImmediate(100*time.Millisecond, timeout, func() (bool, error) {
		if err := rc.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, pod); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return podRunningAndReady(pod)
	})
	return pod, err
}

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

// podRunningAndReady returns true if the pod is running and ready, false if the pod has not
// yet reached those states, returns ErrPodCompleted if the pod has run to completion, or
// an error in any other case.
func podRunningAndReady(pod *corev1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false, ErrPodCompleted
	case corev1.PodFailed:
		return false, fmt.Errorf(pod.Status.ContainerStatuses[0].State.Terminated.Message)
	case corev1.PodRunning:
		conditions := pod.Status.Conditions
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
	return false, nil
}
