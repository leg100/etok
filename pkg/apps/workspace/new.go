package workspace

import (
	"context"
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/k8s"
	stoktyped "github.com/leg100/stok/pkg/k8s/stokclient/typed/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"
)

type NewWorkspace struct {
	*app.Options
	Name                 string
	ServiceAccountClient typedv1.ServiceAccountInterface
	SecretClient         typedv1.SecretInterface
	WorkspaceClient      stoktyped.WorkspaceInterface
	PodClient            typedv1.PodInterface

	// Recall if resources are created so that if error occurs they can be cleaned up
	createdWorkspace      bool
	createdServiceAccount bool
	createdSecret         bool
}

func NewFromOpts(opts *app.Options) app.App {
	return &NewWorkspace{
		Options:              opts,
		Name:                 fmt.Sprintf("%s/%s", opts.Namespace, opts.Workspace),
		ServiceAccountClient: opts.KubeClient().CoreV1().ServiceAccounts(opts.Namespace),
		SecretClient:         opts.KubeClient().CoreV1().Secrets(opts.Namespace),
		WorkspaceClient:      opts.StokClient().StokV1alpha1().Workspaces(opts.Namespace),
		PodClient:            opts.KubeClient().CoreV1().Pods(opts.Namespace),
	}
}

// Create new workspace. First check values of secret and service account flags, if either are empty
// then search for respective resources named "stok" and if they exist, set in the workspace spec
// accordingly. Otherwise use user-supplied values and check they exist. Only then create the
// workspace resource and wait until it is reporting it is healthy, or the timeout expires.
func (n *NewWorkspace) Run(ctx context.Context) error {
	if err := n.run(ctx); err != nil {
		n.cleanup()
		return err
	}
	return nil
}

func (n *NewWorkspace) run(ctx context.Context) error {
	if n.CreateServiceAccount {
		if err := n.createServiceAccountIfMissing(ctx); err != nil {
			return err
		}
	}

	if n.CreateSecret {
		if err := n.createSecretIfMissing(ctx); err != nil {
			return err
		}
	}

	ws, err := n.createWorkspace(ctx)
	if err != nil {
		return err
	}
	n.createdWorkspace = true
	log.Infof("Created workspace %s\n", n.Name)

	// Monitor resources, wait until pod is running and ready
	if err := n.monitor(ctx, ws); err != nil {
		return err
	}

	pod, err := n.PodClient.Get(ctx, ws.PodName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod %s/%s: %w", n.Namespace, n.Workspace, err)
	}

	// Attach to pod, and release hold annotation
	if err = k8s.PodConnect(ctx, n.PodHandler, n.KubeClient(), n.KubeConfig(), pod, n.Out, func() error {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			ws, err := n.WorkspaceClient.Get(ctx, ws.GetName(), metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("getting workspace to delete wait annotation: %w", err)
			}

			k8s.DeleteWaitAnnotationKey(ws)

			_, err = n.WorkspaceClient.Update(ctx, ws, metav1.UpdateOptions{})
			return err
		})
	}); err != nil {
		return err
	}

	return env.NewStokEnv(n.Namespace, n.Workspace).Write(n.Path)
}

func (n *NewWorkspace) cleanup() {
	if n.createdWorkspace {
		n.WorkspaceClient.Delete(context.Background(), n.Workspace, metav1.DeleteOptions{})
	}
	if n.createdSecret {
		n.SecretClient.Delete(context.Background(), n.WorkspaceSpec.SecretName, metav1.DeleteOptions{})
	}
	if n.createdServiceAccount {
		n.ServiceAccountClient.Delete(context.Background(), n.WorkspaceSpec.ServiceAccountName, metav1.DeleteOptions{})
	}
}

func (n *NewWorkspace) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.Workspace,
			Namespace: n.Namespace,
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-operator",

				// Unique name of instance within application
				"app.kubernetes.io/instance": n.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
		Spec: n.WorkspaceSpec,
	}

	ws.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})
	ws.SetDebug(n.Debug)

	return n.StokClient().StokV1alpha1().Workspaces(n.Namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (n *NewWorkspace) createSecretIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := n.WorkspaceSpec.SecretName

	// Check if it exists already
	_, err := n.SecretClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := n.createSecret(ctx, name)
			if err != nil {
				return fmt.Errorf("attempted to create secret: %w", err)
			}
			n.createdSecret = true
		} else {
			return fmt.Errorf("attempted to retrieve secret: %w", err)
		}
	}
	return nil
}

func (n *NewWorkspace) createServiceAccountIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := n.WorkspaceSpec.ServiceAccountName

	// Check if it exists already
	_, err := n.ServiceAccountClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := n.createServiceAccount(ctx, name)
			if err != nil {
				return fmt.Errorf("attempted to create service account: %w", err)
			}
			n.createdServiceAccount = true
		} else {
			return fmt.Errorf("attempted to retrieve service account: %w", err)
		}
	}
	return nil
}

func (n *NewWorkspace) createSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-cli",

				// Unique name of instance within application
				"app.kubernetes.io/instance": n.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
	}

	return n.SecretClient.Create(ctx, secret, metav1.CreateOptions{})
}

func (n *NewWorkspace) createServiceAccount(ctx context.Context, name string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-cli",

				// Unique name of instance within application
				"app.kubernetes.io/instance": n.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
	}

	return n.ServiceAccountClient.Create(ctx, serviceAccount, metav1.CreateOptions{})
}

func (n *NewWorkspace) monitor(ctx context.Context, ws *v1alpha1.Workspace) error {
	errch := make(chan error)
	ready := make(chan struct{})

	// Non-blocking; watch workspace resource, check workspace is healthy
	// TODO: What is the point of this?
	//(&workspaceMonitor{
	//	ws:     ws,
	//	client: n.StokClient(),
	//}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		ws:     ws,
		client: n.KubeClient(),
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
