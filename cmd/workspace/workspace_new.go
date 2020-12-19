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

	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/logstreamer"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watchtools "k8s.io/client-go/tools/watch"
)

const (
	defaultTimeoutWorkspace   = 10 * time.Second
	defaultPodTimeout         = 60 * time.Second
	defaultCacheSize          = "1Gi"
	defaultSecretName         = "etok"
	defaultServiceAccountName = "etok"
)

var (
	errPodTimeout = errors.New("timed out waiting for pod to be ready")
)

type NewOptions struct {
	*cmdutil.Options

	*client.Client

	Path        string
	Namespace   string
	Workspace   string
	KubeContext string

	// etok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec
	// Create a service acccount if it does not exist
	DisableCreateServiceAccount bool
	// Create a secret if it does not exist
	DisableCreateSecret bool
	// Timeout for workspace to be healthy
	TimeoutWorkspace time.Duration

	// Timeout for workspace pod to be ready
	PodTimeout time.Duration

	// Disable default behaviour of deleting resources upon error
	DisableResourceCleanup bool

	// Recall if resources are created so that if error occurs they can be
	// cleaned up
	createdWorkspace      bool
	createdServiceAccount bool
	createdSecret         bool
}

func NewCmd(opts *cmdutil.Options) (*cobra.Command, *NewOptions) {
	o := &NewOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   "new <namespace/workspace>",
		Short: "Create a new etok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.Namespace, o.Workspace, err = env.ValidateAndParse(args[0])
			if err != nil {
				return err
			}

			// Storage class default is nil not empty string (pflags doesn't
			// permit default of nil)
			if !flags.IsFlagPassed(cmd.Flags(), "storage-class") {
				o.WorkspaceSpec.Cache.StorageClass = nil
			}

			o.Client, err = opts.Create(o.KubeContext)
			if err != nil {
				return err
			}

			err = o.Run(cmd.Context())
			if err != nil {
				if !o.DisableResourceCleanup {
					o.cleanup()
				}
			}
			return err
		},
	}

	flags.AddPathFlag(cmd, &o.Path)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)
	flags.AddDisableResourceCleanupFlag(cmd, &o.DisableResourceCleanup)

	cmd.Flags().BoolVar(&o.DisableCreateServiceAccount, "no-create-service-account", o.DisableCreateServiceAccount, "Create service account if missing")
	cmd.Flags().BoolVar(&o.DisableCreateSecret, "no-create-secret", o.DisableCreateSecret, "Create secret if missing")

	cmd.Flags().StringVar(&o.WorkspaceSpec.ServiceAccountName, "service-account", defaultServiceAccountName, "Name of ServiceAccount")
	cmd.Flags().StringVar(&o.WorkspaceSpec.SecretName, "secret", defaultSecretName, "Name of Secret containing credentials")
	cmd.Flags().StringVar(&o.WorkspaceSpec.Cache.Size, "size", defaultCacheSize, "Size of PersistentVolume for cache")
	cmd.Flags().StringVar(&o.WorkspaceSpec.TerraformVersion, "terraform-version", "", "Override terraform version")

	// We want nil to be the default but it doesn't seem like pflags supports
	// that so use empty string and override later (see above)
	o.WorkspaceSpec.Cache.StorageClass = cmd.Flags().String("storage-class", "", "StorageClass of PersistentVolume for cache")

	cmd.Flags().DurationVar(&o.TimeoutWorkspace, "timeout", defaultTimeoutWorkspace, "Time to wait for workspace to be healthy")
	cmd.Flags().DurationVar(&o.PodTimeout, "pod-timeout", defaultPodTimeout, "timeout for pod to be ready")

	cmd.Flags().StringSliceVar(&o.WorkspaceSpec.PrivilegedCommands, "privileged-commands", []string{}, "Set privileged commands")

	return cmd, o
}

func (o *NewOptions) name() string {
	return fmt.Sprintf("%s/%s", o.Namespace, o.Workspace)
}

func (o *NewOptions) Run(ctx context.Context) error {
	if !o.DisableCreateServiceAccount {
		if err := o.createServiceAccountIfMissing(ctx); err != nil {
			return err
		}
	}

	if !o.DisableCreateSecret {
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

	// Wait until container can be streamed from
	fmt.Println("Waiting for workspace pod to be ready...")
	pod, err := o.waitForContainer(ctx, ws)
	if err != nil {
		return err
	}

	// Monitor exit code; non-blocking
	exit := monitors.ExitMonitor(ctx, o.KubeClient, pod.Name, pod.Namespace, controllers.InstallerContainerName)

	if err := logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.Namespace), ws.PodName(), controllers.InstallerContainerName); err != nil {
		return err
	}

	if err := env.NewEtokEnv(o.Namespace, o.Workspace).Write(o.Path); err != nil {
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

func (o *NewOptions) cleanup() {
	if o.createdWorkspace {
		o.WorkspacesClient(o.Namespace).Delete(context.Background(), o.Workspace, metav1.DeleteOptions{})
	}
	if o.createdSecret {
		o.SecretsClient(o.Namespace).Delete(context.Background(), o.WorkspaceSpec.SecretName, metav1.DeleteOptions{})
	}
	if o.createdServiceAccount {
		o.ServiceAccountsClient(o.Namespace).Delete(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.DeleteOptions{})
	}
}

func (o *NewOptions) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Workspace,
			Namespace: o.Namespace,
		},
		Spec: v1alpha1.WorkspaceSpec{
			SecretName:         o.WorkspaceSpec.SecretName,
			ServiceAccountName: o.WorkspaceSpec.ServiceAccountName,
			Cache: v1alpha1.WorkspaceCacheSpec{
				StorageClass: o.WorkspaceSpec.Cache.StorageClass,
				Size:         o.WorkspaceSpec.Cache.Size,
			},
			PrivilegedCommands: o.WorkspaceSpec.PrivilegedCommands,
			TerraformVersion:   o.WorkspaceSpec.TerraformVersion,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(ws)
	// Permit filtering secrets by workspace
	labels.SetLabel(ws, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(ws, labels.WorkspaceComponent)

	ws.Spec.Verbosity = o.Verbosity

	return o.WorkspacesClient(o.Namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (o *NewOptions) createSecretIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := o.WorkspaceSpec.SecretName

	// Check if it exists already
	_, err := o.SecretsClient(o.Namespace).Get(ctx, name, metav1.GetOptions{})
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

func (o *NewOptions) createServiceAccountIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := o.WorkspaceSpec.ServiceAccountName

	// Check if it exists already
	_, err := o.ServiceAccountsClient(o.Namespace).Get(ctx, name, metav1.GetOptions{})
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

func (o *NewOptions) createSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	// Set etok's common labels
	labels.SetCommonLabels(secret)
	// Permit filtering secrets by workspace
	labels.SetLabel(secret, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(secret, labels.WorkspaceComponent)

	return o.SecretsClient(o.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (o *NewOptions) createServiceAccount(ctx context.Context, name string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	// Set etok's common labels
	labels.SetCommonLabels(serviceAccount)
	// Permit filtering service accounts by workspace
	labels.SetLabel(serviceAccount, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(serviceAccount, labels.WorkspaceComponent)

	return o.ServiceAccountsClient(o.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
}

// waitForContainer returns true once the installer container can be streamed
// from
func (o *NewOptions) waitForContainer(ctx context.Context, ws *v1alpha1.Workspace) (*corev1.Pod, error) {
	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: ws.PodName(), Namespace: ws.Namespace}
	hdlr := handlers.ContainerReady(ws.PodName(), controllers.InstallerContainerName, true, false)

	ctx, cancel := context.WithTimeout(ctx, o.PodTimeout)
	defer cancel()

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return nil, errPodTimeout
		}
		return nil, err
	}
	return event.Object.(*corev1.Pod), err
}

// waitForWorkspaceInitializing waits until the workspace reports it is
// initializing (or ready)
func (o *NewOptions) waitForWorkspaceInitializing(ctx context.Context, ws *v1alpha1.Workspace) error {
	lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: ws.Name, Namespace: ws.Namespace}
	hdlr := handlers.ContainerReady(ws.PodName(), controllers.InstallerContainerName, true, false)

	ctx, cancel := context.WithTimeout(ctx, o.PodTimeout)
	defer cancel()

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return nil, errPodTimeout
		}
		return nil, err
	}
	return event.Object.(*corev1.Pod), err
}
