package workspace

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/util"
	"github.com/leg100/stok/version"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type NewWorkspace struct {
	Name              string
	Namespace         string
	Path              string
	Timeout           time.Duration
	TimeoutPod        time.Duration
	Context           string
	WorkspaceSpec     v1alpha1.WorkspaceSpec
	SecretSet         bool
	ServiceAccountSet bool

	Debug bool
	Out   io.Writer
}

// Checks if user has specified a resource dependency and if so, that it exists. If it does not,
// then throw an error.
// If user has not specified dependency, i.e. they have left it to the default "stok", then if it
// does not exist, set the dependency name to "" and don't throw an error.
func checkResourceExistsIfSet(name *string, isSet bool, get func(string) (runtime.Object, error)) error {
	resource, err := get(*name)
	if errors.IsNotFound(err) {
		if isSet {
			return fmt.Errorf("specified %T not found: %s", resource, *name)
		} else {
			log.Debugf("Default %T %s not found; skipping", resource, *name)
			*name = ""
			return nil
		}
	}
	return err
}

// Create new workspace. First check values of secret and service account flags, if either are empty
// then search for respective resources named "stok" and if they exist, set in the workspace spec
// accordingly. Otherwise use user-supplied values and check they exist. Only then create the
// workspace resource and wait until it is reporting it is healthy, or the timeout expires.
func (t *NewWorkspace) Run(ctx context.Context) error {
	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	kc, err := k8s.KubeClient()
	if err != nil {
		return err
	}

	err = checkResourceExistsIfSet(&t.WorkspaceSpec.SecretName, t.SecretSet, func(name string) (runtime.Object, error) {
		return kc.CoreV1().Secrets(t.Namespace).Get(ctx, name, metav1.GetOptions{})
	})
	if err != nil {
		return err
	}

	err = checkResourceExistsIfSet(&t.WorkspaceSpec.ServiceAccountName, t.ServiceAccountSet, func(name string) (runtime.Object, error) {
		return kc.CoreV1().ServiceAccounts(t.Namespace).Get(ctx, name, metav1.GetOptions{})
	})
	if err != nil {
		return err
	}

	ws, err := t.createWorkspace(ctx, sc)
	if err != nil {
		return err
	}
	deleteWorkspace := func() {
		sc.StokV1alpha1().Workspaces(t.Namespace).Delete(ctx, ws.GetName(), metav1.DeleteOptions{})
	}
	log.WithFields(log.Fields{"namespace": t.Namespace, "workspace": t.Name}).Info("created workspace")

	// Monitor resources, wait until pod is running and ready
	if err := t.monitor(ctx, sc, kc, ws); err != nil {
		deleteWorkspace()
		return err
	}

	// Attach to pod, and release hold annotation
	if err = k8s.PodConnect(ctx, kc, t.Namespace, ws.PodName(), func() error {
		wsclient := sc.StokV1alpha1().Workspaces(t.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			ws, err := wsclient.Get(ctx, ws.GetName(), metav1.GetOptions{})
			if err != nil {
				return err
			}

			k8s.DeleteWaitAnnotationKey(ws)

			_, err = wsclient.Update(ctx, ws, metav1.UpdateOptions{})
			return err
		})
	}); err != nil {
		deleteWorkspace()
		return err
	}

	if err := util.WriteEnvironmentFile(t.Path, t.Namespace, t.Name); err != nil {
		return err
	}

	return nil
}

func (t *NewWorkspace) createWorkspace(ctx context.Context, sc stokclient.Interface) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-operator",

				// Unique name of instance within application
				"app.kubernetes.io/instance": t.Name,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
		Spec: t.WorkspaceSpec,
	}

	ws.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})
	ws.SetDebug(t.Debug)

	return sc.StokV1alpha1().Workspaces(t.Namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (t *NewWorkspace) monitor(ctx context.Context, sc stokclient.Interface, kc kubernetes.Interface, ws *v1alpha1.Workspace) error {
	errch := make(chan error)
	ready := make(chan struct{})

	// Non-blocking; watch workspace resource, check workspace is healthy
	// TODO: What is the point of this?
	(&workspaceMonitor{
		ws:     ws,
		client: sc,
	}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		ws:     ws,
		client: kc,
	}).monitor(ctx, errch, ready)

	// Wait for pod to be ready
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errch:
		return err
	}
}
