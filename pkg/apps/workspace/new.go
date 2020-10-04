package workspace

import (
	"context"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/apps"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/options"
	"github.com/leg100/stok/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type NewWorkspace struct {
	Name          string
	Namespace     string
	Path          string
	Timeout       time.Duration
	TimeoutPod    time.Duration
	WorkspaceSpec v1alpha1.WorkspaceSpec

	StokClient stokclient.Interface
	KubeClient kubernetes.Interface

	Debug bool
}

func NewFromOptions(ctx context.Context, opts *options.StokOptions) (apps.App, error) {
	return &NewWorkspace{
		Name:          opts.Name,
		Namespace:     opts.Namespace,
		Path:          opts.Path,
		Timeout:       opts.TimeoutWorkspace,
		TimeoutPod:    opts.TimeoutWorkspacePod,
		WorkspaceSpec: opts.WorkspaceSpec,
		StokClient:    opts.StokClient,
		KubeClient:    opts.KubeClient,
		Debug:         opts.Debug,
	}, nil
}

// Create new workspace. First check values of secret and service account flags, if either are empty
// then search for respective resources named "stok" and if they exist, set in the workspace spec
// accordingly. Otherwise use user-supplied values and check they exist. Only then create the
// workspace resource and wait until it is reporting it is healthy, or the timeout expires.
func (nws *NewWorkspace) Run(ctx context.Context) error {
	ws, err := nws.createWorkspace(ctx)
	if err != nil {
		return err
	}
	deleteWorkspace := func() {
		nws.StokClient.StokV1alpha1().Workspaces(nws.Namespace).Delete(ctx, ws.GetName(), metav1.DeleteOptions{})
	}
	log.WithFields(log.Fields{"namespace": nws.Namespace, "workspace": nws.Name}).Info("created workspace")

	// Monitor resources, wait until pod is running and ready
	if err := nws.monitor(ctx, ws); err != nil {
		deleteWorkspace()
		return err
	}

	// Attach to pod, and release hold annotation
	if err = k8s.PodConnect(ctx, nws.KubeClient, nws.Namespace, ws.PodName(), func() error {
		wsclient := nws.StokClient.StokV1alpha1().Workspaces(nws.Namespace)
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

	return env.NewStokEnv(nws.Namespace, nws.Name).Write(nws.Path)
}

func (nws *NewWorkspace) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nws.Name,
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
				"app.kubernetes.io/instance": nws.Name,

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

	return nws.StokClient.StokV1alpha1().Workspaces(nws.Namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (nws *NewWorkspace) monitor(ctx context.Context, ws *v1alpha1.Workspace) error {
	errch := make(chan error)
	ready := make(chan struct{})

	// Non-blocking; watch workspace resource, check workspace is healthy
	// TODO: What is the point of this?
	(&workspaceMonitor{
		ws:     ws,
		client: nws.StokClient,
	}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		ws:     ws,
		client: nws.KubeClient,
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
