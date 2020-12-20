package workspace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/controllers"
	"github.com/leg100/etok/pkg/handlers"
	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/monitors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/logstreamer"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watchtools "k8s.io/client-go/tools/watch"
)

const (
	defaultReconcileTimeout   = 10 * time.Second
	defaultPodTimeout         = 60 * time.Second
	defaultCacheSize          = "1Gi"
	defaultSecretName         = "etok"
	defaultServiceAccountName = "etok"
)

var (
	errPodTimeout       = errors.New("timed out waiting for pod to be ready")
	errReconcileTimeout = errors.New("timed out waiting for workspace to be reconciled")
)

type newOptions struct {
	*cmdutil.Options

	*client.Client

	path        string
	namespace   string
	workspace   string
	kubeContext string

	// etok Workspace's workspaceSpec
	workspaceSpec v1alpha1.WorkspaceSpec
	// Create a service acccount if it does not exist
	disableCreateServiceAccount bool
	// Create a secret if it does not exist
	disableCreateSecret bool

	// Timeout for resource to be reconciled (at least once)
	reconcileTimeout time.Duration

	// Timeout for workspace pod to be ready
	podTimeout time.Duration

	// Disable default behaviour of deleting resources upon error
	disableResourceCleanup bool

	// Recall if resources are created so that if error occurs they can be
	// cleaned up
	createdWorkspace      bool
	createdServiceAccount bool
	createdSecret         bool

	// For testing purposes toggle obj having been reconciled
	reconciled bool
}

func newCmd(opts *cmdutil.Options) (*cobra.Command, *newOptions) {
	o := &newOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   "new <namespace/workspace>",
		Short: "Create a new etok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.namespace, o.workspace, err = env.ValidateAndParse(args[0])
			if err != nil {
				return err
			}

			// Storage class default is nil not empty string (pflags doesn't
			// permit default of nil)
			if !flags.IsFlagPassed(cmd.Flags(), "storage-class") {
				o.workspaceSpec.Cache.StorageClass = nil
			}

			o.Client, err = opts.Create(o.kubeContext)
			if err != nil {
				return err
			}

			err = o.run(cmd.Context())
			if err != nil {
				if !o.disableResourceCleanup {
					o.cleanup()
				}
			}
			return err
		},
	}

	flags.AddPathFlag(cmd, &o.path)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)
	flags.AddDisableResourceCleanupFlag(cmd, &o.disableResourceCleanup)

	cmd.Flags().BoolVar(&o.disableCreateServiceAccount, "no-create-service-account", o.disableCreateServiceAccount, "Create service account if missing")
	cmd.Flags().BoolVar(&o.disableCreateSecret, "no-create-secret", o.disableCreateSecret, "Create secret if missing")

	cmd.Flags().StringVar(&o.workspaceSpec.ServiceAccountName, "service-account", defaultServiceAccountName, "Name of ServiceAccount")
	cmd.Flags().StringVar(&o.workspaceSpec.SecretName, "secret", defaultSecretName, "Name of Secret containing credentials")
	cmd.Flags().StringVar(&o.workspaceSpec.Cache.Size, "size", defaultCacheSize, "Size of PersistentVolume for cache")
	cmd.Flags().StringVar(&o.workspaceSpec.TerraformVersion, "terraform-version", "", "Override terraform version")

	// We want nil to be the default but it doesn't seem like pflags supports
	// that so use empty string and override later (see above)
	o.workspaceSpec.Cache.StorageClass = cmd.Flags().String("storage-class", "", "StorageClass of PersistentVolume for cache")

	cmd.Flags().DurationVar(&o.reconcileTimeout, "reconcile-timeout", defaultReconcileTimeout, "timeout for resource to be reconciled")
	cmd.Flags().DurationVar(&o.podTimeout, "pod-timeout", defaultPodTimeout, "timeout for pod to be ready")

	cmd.Flags().StringSliceVar(&o.workspaceSpec.PrivilegedCommands, "privileged-commands", []string{}, "Set privileged commands")

	return cmd, o
}

func (o *newOptions) name() string {
	return fmt.Sprintf("%s/%s", o.namespace, o.workspace)
}

func (o *newOptions) run(ctx context.Context) error {
	if !o.disableCreateServiceAccount {
		if err := o.createServiceAccountIfMissing(ctx); err != nil {
			return err
		}
	}

	if !o.disableCreateSecret {
		if err := o.createSecretIfMissing(ctx); err != nil {
			return err
		}
	}

	ws, err := o.createWorkspace(ctx)
	if err != nil {
		return err
	}
	o.createdWorkspace = true
	fmt.Printf("Created workspace %s\n", o.name())

	g, gctx := errgroup.WithContext(ctx)

	fmt.Println("Waiting for workspace pod to be ready...")
	podch := make(chan *corev1.Pod, 1)
	g.Go(func() error {
		return o.waitForContainer(gctx, ws, podch)
	})

	// Wait for resource to have been successfully reconciled at least once
	// within the ReconcileTimeout (If we don't do this and the operator is
	// either not installed or malfunctioning then the user would be none the
	// wiser until the much longer PodTimeout had expired).
	g.Go(func() error {
		return o.waitForReconcile(gctx, ws)
	})

	// Wait for both workspace to have been reconciled and for its pod container
	// to be ready
	if err := g.Wait(); err != nil {
		return err
	}

	// Receive ready pod
	pod := <-podch

	// Monitor exit code; non-blocking
	exit := monitors.ExitMonitor(ctx, o.KubeClient, pod.Name, pod.Namespace, controllers.InstallerContainerName)

	if err := logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.namespace), ws.PodName(), controllers.InstallerContainerName); err != nil {
		return err
	}

	if err := env.NewEtokEnv(o.namespace, o.workspace).Write(o.path); err != nil {
		return err
	}

	// Return container's exit code
	select {
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for exit code")
	case code := <-exit:
		return code
	}
}

func (o *newOptions) cleanup() {
	if o.createdWorkspace {
		o.WorkspacesClient(o.namespace).Delete(context.Background(), o.workspace, metav1.DeleteOptions{})
	}
	if o.createdSecret {
		o.SecretsClient(o.namespace).Delete(context.Background(), o.workspaceSpec.SecretName, metav1.DeleteOptions{})
	}
	if o.createdServiceAccount {
		o.ServiceAccountsClient(o.namespace).Delete(context.Background(), o.workspaceSpec.ServiceAccountName, metav1.DeleteOptions{})
	}
}

func (o *newOptions) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.workspace,
			Namespace: o.namespace,
		},
		Spec: v1alpha1.WorkspaceSpec{
			SecretName:         o.workspaceSpec.SecretName,
			ServiceAccountName: o.workspaceSpec.ServiceAccountName,
			Cache: v1alpha1.WorkspaceCacheSpec{
				StorageClass: o.workspaceSpec.Cache.StorageClass,
				Size:         o.workspaceSpec.Cache.Size,
			},
			PrivilegedCommands: o.workspaceSpec.PrivilegedCommands,
			TerraformVersion:   o.workspaceSpec.TerraformVersion,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(ws)
	// Permit filtering secrets by workspace
	labels.SetLabel(ws, labels.Workspace(o.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(ws, labels.WorkspaceComponent)

	ws.Spec.Verbosity = o.Verbosity

	// For testing purposes mimic obj having been reconciled
	ws.Status.Reconciled = o.reconciled

	return o.WorkspacesClient(o.namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (o *newOptions) createSecretIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := o.workspaceSpec.SecretName

	// Check if it exists already
	_, err := o.SecretsClient(o.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			_, err := o.createSecret(ctx, name)
			if err != nil {
				return fmt.Errorf("attempted to create secret: %w", err)
			}
			o.createdSecret = true
		} else {
			return fmt.Errorf("attempted to retrieve secret: %w", err)
		}
	}
	return nil
}

func (o *newOptions) createServiceAccountIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := o.workspaceSpec.ServiceAccountName

	// Check if it exists already
	_, err := o.ServiceAccountsClient(o.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			_, err := o.createServiceAccount(ctx, name)
			if err != nil {
				return fmt.Errorf("attempted to create service account: %w", err)
			}
			o.createdServiceAccount = true
		} else {
			return fmt.Errorf("attempted to retrieve service account: %w", err)
		}
	}
	return nil
}

func (o *newOptions) createSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	// Set etok's common labels
	labels.SetCommonLabels(secret)
	// Permit filtering secrets by workspace
	labels.SetLabel(secret, labels.Workspace(o.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(secret, labels.WorkspaceComponent)

	return o.SecretsClient(o.namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (o *newOptions) createServiceAccount(ctx context.Context, name string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	// Set etok's common labels
	labels.SetCommonLabels(serviceAccount)
	// Permit filtering service accounts by workspace
	labels.SetLabel(serviceAccount, labels.Workspace(o.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(serviceAccount, labels.WorkspaceComponent)

	return o.ServiceAccountsClient(o.namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
}

// waitForContainer returns true once the installer container can be streamed
// from
func (o *newOptions) waitForContainer(ctx context.Context, ws *v1alpha1.Workspace, podch chan<- *corev1.Pod) error {
	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: ws.PodName(), Namespace: ws.Namespace}
	hdlr := handlers.ContainerReady(ws.PodName(), controllers.InstallerContainerName, true, false)

	ctx, cancel := context.WithTimeout(ctx, o.podTimeout)
	defer cancel()

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return errPodTimeout
		}
		return err
	}
	podch <- event.Object.(*corev1.Pod)
	return nil
}

// waitForReconcile waits for the workspace resource to be reconciled.
func (o *newOptions) waitForReconcile(ctx context.Context, ws *v1alpha1.Workspace) error {
	lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: ws.Name, Namespace: ws.Namespace}
	hdlr := handlers.Reconciled(ws)

	ctx, cancel := context.WithTimeout(ctx, o.reconcileTimeout)
	defer cancel()

	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return errReconcileTimeout
		}
		return err
	}
	return nil
}
