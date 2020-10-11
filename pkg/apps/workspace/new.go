package workspace

import (
	"context"
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/podhandler"
	"github.com/leg100/stok/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type NewWorkspace struct {
	*app.Options
	PodHandler podhandler.Interface
}

func NewFromOpts(opts *app.Options) app.App {
	return &NewWorkspace{
		Options: opts,
		PodHandler: &podhandler.PodHandler{},
	}
}

// Create new workspace. First check values of secret and service account flags, if either are empty
// then search for respective resources named "stok" and if they exist, set in the workspace spec
// accordingly. Otherwise use user-supplied values and check they exist. Only then create the
// workspace resource and wait until it is reporting it is healthy, or the timeout expires.
func (nws *NewWorkspace) Run(ctx context.Context) error {
	if err := nws.run(ctx); err != nil {
		// Delete workspace upon error
		nws.StokClient().StokV1alpha1().Workspaces(nws.Namespace).Delete(ctx, nws.Workspace, metav1.DeleteOptions{})
		return err
	}
	return nil
}

func (nws *NewWorkspace) run(ctx context.Context) error {
	ws, err := nws.createWorkspace(ctx)
	log.Infof("Created workspace %s/%s\n", nws.Namespace, nws.Workspace)

	// Monitor resources, wait until pod is running and ready
	if err := nws.monitor(ctx, ws); err != nil {
		return err
	}

	pod, err := nws.KubeClient().CoreV1().Pods(nws.Namespace).Get(ctx, nws.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod %s/%s: %w", nws.Namespace, nws.Name, err)
	}

	// Attach to pod, and release hold annotation
	if err = k8s.PodConnect(ctx, nws.PodHandler, nws.KubeClient(), nws.KubeConfig, pod, nws.Out, func() error {
		wsclient := nws.StokClient().StokV1alpha1().Workspaces(nws.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			ws, err := wsclient.Get(ctx, ws.GetName(), metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("getting workspace to delete wait annotation: %w", err)
			}

			k8s.DeleteWaitAnnotationKey(ws)

			_, err = wsclient.Update(ctx, ws, metav1.UpdateOptions{})
			return err
		})
	}); err != nil {
		return err
	}

	return env.NewStokEnv(nws.Namespace, nws.Workspace).Write(nws.Path)
}

func (nws *NewWorkspace) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nws.Workspace,
			Namespace: nws.Namespace,
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-operator",

				// Unique name of instance within application
				"app.kubernetes.io/instance": nws.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
		Spec: nws.WorkspaceSpec,
	}

	ws.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})
	ws.SetDebug(nws.Debug)

	return nws.StokClient().StokV1alpha1().Workspaces(nws.Namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (nws *NewWorkspace) monitor(ctx context.Context, ws *v1alpha1.Workspace) error {
	errch := make(chan error)
	ready := make(chan struct{})

	// Non-blocking; watch workspace resource, check workspace is healthy
	// TODO: What is the point of this?
	(&workspaceMonitor{
		ws:     ws,
		client: nws.StokClient(),
	}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		ws:     ws,
		client: nws.KubeClient(),
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
