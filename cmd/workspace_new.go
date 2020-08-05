package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/v1alpha1"
	v1alpha1types "github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type newWorkspaceCmd struct {
	Name           string
	Namespace      string
	Path           string
	CacheSize      string
	StorageClass   string
	NoSecret       bool
	Secret         string
	ServiceAccount string
	Timeout        time.Duration
	TimeoutClient  time.Duration
	TimeoutPod     time.Duration
	Context        string
	Backend        v1alpha1.BackendSpec

	factory k8s.FactoryInterface
	debug   bool
	out     io.Writer
	cmd     *cobra.Command
}

func newNewWorkspaceCmd(f k8s.FactoryInterface, out io.Writer) *cobra.Command {
	cc := &newWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "new <workspace>",
		Short: "Create a new stok workspace",
		Long:  "Deploys a Workspace resource",
		Args:  cobra.ExactArgs(1),
		RunE:  cc.doNewWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "default", "Kubernetes namespace of workspace")
	cc.cmd.Flags().StringVar(&cc.ServiceAccount, "service-account", "", "Name of ServiceAccount")
	cc.cmd.Flags().StringVar(&cc.Secret, "secret", "", "Name of Secret containing credentials")
	cc.cmd.Flags().StringVar(&cc.CacheSize, "size", "1Gi", "Size of PersistentVolume for cache")
	cc.cmd.Flags().StringVar(&cc.StorageClass, "storage-class", "", "StorageClass of PersistentVolume for cache")
	cc.cmd.Flags().DurationVar(&cc.Timeout, "timeout", 10*time.Second, "Time to wait for workspace to be healthy")
	cc.cmd.Flags().DurationVar(&cc.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
	cc.cmd.Flags().DurationVar(&cc.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
	cc.cmd.Flags().StringVar(&cc.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

	cc.cmd.Flags().StringVar(&cc.Backend.Type, "backend-type", "local", "Set backend type")
	cc.cmd.Flags().StringToStringVar(&cc.Backend.Config, "backend-config", map[string]string{}, "Set backend config (command separated key values, e.g. bucket=gcs,prefix=dev")

	// Add flags registered by imported packages (controller-runtime)
	cc.cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cc.factory = f
	cc.out = out

	return cc.cmd
}

func CheckResourceExists(rc client.Client, name, namespace string, obj runtime.Object) (bool, error) {
	nn := types.NamespacedName{Name: name, Namespace: namespace}
	if err := rc.Get(context.TODO(), nn, obj); err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}
	return true, nil
}

// Create new workspace. First check values of secret and service account flags, if either are empty
// then search for respective resources named "stok" and if they exist, set in the workspace spec
// accordingly. Otherwise use user-supplied values and check they exist. Only then create the
// workspace resource and wait until it is reporting it is healthy, or the timeout expires.
func (t *newWorkspaceCmd) doNewWorkspace(cmd *cobra.Command, args []string) error {
	debug, err := cmd.InheritedFlags().GetBool("debug")
	if err != nil {
		return err
	}
	t.debug = debug

	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Name = args[0]

	// Controller-runtime client for constructing workspace resource
	rc, err := t.factory.NewClient(scheme.Scheme, t.Context)
	if err != nil {
		return err
	}

	if t.Secret != "" {
		// Secret specified; check that it exists and if not found then error
		found, err := CheckResourceExists(rc, t.Secret, t.Namespace, &corev1.Secret{})
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("secret '%s' not found", t.Secret)
		}
	} else {
		// Secret unspecified, check if resource called 'stok' exists and if so, use that
		found, err := CheckResourceExists(rc, "stok", t.Namespace, &corev1.Secret{})
		if err != nil {
			return err
		}
		if found {
			t.Secret = "stok"
			log.Info("Found default secret...")
		}
	}

	if t.ServiceAccount != "" {
		// Service account specified; check that it exists and if not found then error
		found, err := CheckResourceExists(rc, t.ServiceAccount, t.Namespace, &corev1.ServiceAccount{})
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("service account '%s' not found", t.ServiceAccount)
		}
	} else {
		// Service account unspecified, check if resource called 'stok' exists and if so, use that
		found, err := CheckResourceExists(rc, "stok", t.Namespace, &corev1.ServiceAccount{})
		if err != nil {
			return err
		}
		if found {
			t.ServiceAccount = "stok"
			log.Info("Found default service account...")
		}
	}

	ws, err := t.createWorkspace(rc)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"namespace": t.Namespace,
		"workspace": t.Name,
	}).Info("created workspace")

	if err := t.manageNewWorkspace(rc, ws); err != nil {
		// Upon error, clean up (probably broken) workspace
		log.Info("deleting workspace...")
		rc.Delete(context.TODO(), ws)
		return err
	}

	return nil
}

func (t *newWorkspaceCmd) manageNewWorkspace(rc k8s.Client, ws *v1alpha1.Workspace) error {
	// Wait until Workspace's healthy condition is true
	// TODO: parameterize poll interval
	err := wait.PollImmediate(100*time.Millisecond, t.Timeout, func() (bool, error) {
		if err := rc.Get(context.TODO(), types.NamespacedName{Namespace: t.Namespace, Name: t.Name}, ws); err != nil {
			return false, err
		}
		conditions := ws.Status.Conditions
		if conditions == nil {
			return false, nil
		}
		for i := range conditions {
			if conditions[i].Type == v1alpha1.ConditionHealthy {
				log.WithFields(log.Fields{
					"workspace": ws.GetName(),
					"namespace": ws.GetNamespace(),
					"reason":    conditions[i].Reason,
				}).Debug("Checking health")

				if conditions[i].Status == corev1.ConditionTrue {
					return true, nil
				}
				if conditions[i].Status == corev1.ConditionFalse {
					return false, fmt.Errorf(conditions[i].Message)
				}
			}
		}
		return false, nil
	})
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return WorkspaceTimeoutErr
		}
		return err
	}

	log.WithFields(log.Fields{
		"pod":       ws.PodName(),
		"namespace": t.Namespace,
	}).Debug("awaiting readiness")
	pod, err := waitUntilPodInitialisedAndRunning(rc, &corev1.Pod{}, t.Namespace, ws.PodName(), t.TimeoutPod)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"pod":       ws.PodName(),
		"namespace": t.Namespace,
	}).Debug("retrieve log stream")
	logs, err := rc.GetLogs(t.Namespace, ws.PodName(), &corev1.PodLogOptions{Follow: true, Container: "runner"})
	if err != nil {
		return err
	}
	defer logs.Close()

	// Attach to pod tty
	done := make(chan error)
	go func() {
		log.WithFields(log.Fields{
			"pod": ws.PodName(),
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
		objKey := types.NamespacedName{Name: ws.GetName(), Namespace: t.Namespace}
		err = rc.Get(context.TODO(), objKey, ws)
		if err != nil {
			done <- err
		} else {
			// Delete annotation WaitAnnotationKey, giving the runner the signal to start
			annotations := ws.GetAnnotations()
			delete(annotations, v1alpha1.WaitAnnotationKey)
			ws.SetAnnotations(annotations)

			return rc.Update(context.TODO(), ws, &client.UpdateOptions{})
		}
		return nil
	})

	err = <-done
	if err != nil {
		return err
	}

	if err := writeEnvironmentFile(t.Path, t.Namespace, t.Name); err != nil {
		return err
	}

	return nil
}

var WorkspaceTimeoutErr = fmt.Errorf("timed out waiting for workspace to be in a healthy condition")

func (t *newWorkspaceCmd) createWorkspace(rc client.Client) (*v1alpha1types.Workspace, error) {
	ws := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app": "stok",
			},
		},
		Spec: v1alpha1types.WorkspaceSpec{
			Backend:       t.Backend,
			TimeoutClient: t.TimeoutClient.String(),
		},
	}

	ws.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})
	ws.SetDebug(t.debug)

	if !t.NoSecret {
		ws.Spec.SecretName = t.Secret
	}

	if t.ServiceAccount != "" {
		ws.Spec.ServiceAccountName = t.ServiceAccount
	}

	if t.CacheSize != "" {
		ws.Spec.Cache.Size = t.CacheSize
	}

	if t.StorageClass != "" {
		ws.Spec.Cache.StorageClass = t.StorageClass
	}

	err := rc.Create(context.TODO(), ws)
	return ws, err
}

func waitUntilPodInitialisedAndRunning(rc k8s.Client, pod *corev1.Pod, namespace, name string, timeout time.Duration) (*corev1.Pod, error) {
	err := wait.Poll(100*time.Millisecond, timeout, func() (bool, error) {
		if err := rc.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, pod); err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return podInitialisedAndRunning(pod)
	})
	return pod, err
}

// podRunningAndReady returns true if the pod is running and ready, false if the pod has not
// yet reached those states, returns ErrPodCompleted if the pod has run to completion, or
// an error in any other case.
func podInitialisedAndRunning(pod *corev1.Pod) (bool, error) {
	if len(pod.Status.InitContainerStatuses) > 0 {
		if pod.Status.InitContainerStatuses[0].State.Running != nil {
			return true, nil
		}
	}
	return false, nil
}
