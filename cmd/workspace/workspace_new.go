package workspace

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/handlers"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/labels"
	"github.com/leg100/stok/pkg/runner"
	"github.com/spf13/cobra"

	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/logstreamer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	defaultTimeoutWorkspace    = 10 * time.Second
	defaultTimeoutWorkspacePod = 60 * time.Second
	defaultBackendType         = "local"
	defaultCacheSize           = "1Gi"
	defaultSecretName          = "stok"
	defaultServiceAccountName  = "stok"
)

type NewOptions struct {
	*cmdutil.Options

	*client.Client

	Path        string
	Namespace   string
	Workspace   string
	KubeContext string

	// Stok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec
	// Create a service acccount if it does not exist
	DisableCreateServiceAccount bool
	// Create a secret if it does not exist
	DisableCreateSecret bool
	// Timeout for workspace to be healthy
	TimeoutWorkspace time.Duration
	// Timeout for workspace pod to be running and ready
	TimeoutWorkspacePod time.Duration
	// Timeout for wait for handshake
	HandshakeTimeout time.Duration
	// Disable default behaviour of deleting resources upon error
	DisableResourceCleanup bool

	// Disable TTY detection
	DisableTTY bool

	// Recall if resources are created so that if error occurs they can be cleaned up
	createdWorkspace      bool
	createdServiceAccount bool
	createdSecret         bool
}

func NewCmd(opts *cmdutil.Options) (*cobra.Command, *NewOptions) {
	o := &NewOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   "new <namespace/workspace>",
		Short: "Create a new stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.Namespace, o.Workspace, err = env.ValidateAndParse(args[0])
			if err != nil {
				return err
			}

			o.Client, err = opts.Create(o.KubeContext)
			if err != nil {
				return err
			}

			return o.Run(cmd.Context())
		},
	}

	flags.AddPathFlag(cmd, &o.Path)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)

	cmd.Flags().BoolVar(&o.DisableResourceCleanup, "no-cleanup", o.DisableResourceCleanup, "Do not delete kubernetes resources upon error")
	cmd.Flags().BoolVar(&o.DisableCreateServiceAccount, "no-create-service-account", o.DisableCreateServiceAccount, "Create service account if missing")
	cmd.Flags().BoolVar(&o.DisableCreateSecret, "no-create-secret", o.DisableCreateSecret, "Create secret if missing")

	cmd.Flags().StringVar(&o.WorkspaceSpec.ServiceAccountName, "service-account", defaultServiceAccountName, "Name of ServiceAccount")
	cmd.Flags().StringVar(&o.WorkspaceSpec.SecretName, "secret", defaultSecretName, "Name of Secret containing credentials")
	cmd.Flags().StringVar(&o.WorkspaceSpec.Cache.Size, "size", defaultCacheSize, "Size of PersistentVolume for cache")
	cmd.Flags().StringVar(&o.WorkspaceSpec.Cache.StorageClass, "storage-class", "", "StorageClass of PersistentVolume for cache")
	cmd.Flags().StringVar(&o.WorkspaceSpec.Backend.Type, "backend-type", defaultBackendType, "Set backend type")
	cmd.Flags().StringToStringVar(&o.WorkspaceSpec.Backend.Config, "backend-config", map[string]string{}, "Set backend config (command separated key values, e.g. bucket=gcs,prefix=dev")
	cmd.Flags().DurationVar(&o.HandshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "Timeout waiting for handshake")

	cmd.Flags().DurationVar(&o.TimeoutWorkspace, "timeout", defaultTimeoutWorkspace, "Time to wait for workspace to be healthy")
	cmd.Flags().DurationVar(&o.TimeoutWorkspacePod, "timeout-pod", defaultTimeoutWorkspacePod, "timeout for pod to be ready and running")

	cmd.Flags().StringSliceVar(&o.WorkspaceSpec.PrivilegedCommands, "privileged-commands", []string{}, "Set privileged commands")

	cmd.Flags().BoolVar(&o.DisableTTY, "no-tty", false, "disable tty")

	return cmd, o
}

// TODO: refactor to use a wrapper function, i.e. cleanupOnError()
func (o *NewOptions) Run(ctx context.Context) error {
	if err := o.run(ctx); err != nil {
		if !o.DisableResourceCleanup {
			o.cleanup()
		}
		return err
	}
	return nil
}

func (o *NewOptions) name() string {
	return fmt.Sprintf("%s/%s", o.Namespace, o.Workspace)
}

func (o *NewOptions) run(ctx context.Context) error {
	isTTY := !o.DisableTTY && detectTTY(o.In)

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

	ws, err := o.createWorkspace(ctx, isTTY)
	if err != nil {
		return err
	}
	o.createdWorkspace = true
	log.Infof("Created workspace %s\n", o.name())

	// Wait until container can be attached/streamed to/from
	pod, err := o.waitForContainer(ctx, ws, isTTY)
	if err != nil {
		return err
	}

	// Monitor exit code; non-blocking
	exit := runner.ExitMonitor(ctx, o.KubeClient, pod.Name, pod.Namespace)

	if isTTY {
		log.Debug("Attaching to pod")
		if err := o.AttachFunc(o.Out, *o.Config, pod, o.In.(*os.File), cmdutil.HandshakeString, runner.ContainerName); err != nil {
			return err
		}
	} else {
		log.Debug("Retrieving pod's log stream")
		if err := logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.Namespace), ws.PodName(), runner.ContainerName); err != nil {
			return err
		}
	}

	if err := env.NewStokEnv(o.Namespace, o.Workspace).Write(o.Path); err != nil {
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

func (o *NewOptions) createWorkspace(ctx context.Context, isTTY bool) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Workspace,
			Namespace: o.Namespace,
		},
		Spec: o.WorkspaceSpec,
	}
	// Set stok's common labels
	labels.SetCommonLabels(ws)
	// Permit filtering secrets by workspace
	labels.SetLabel(ws, labels.Workspace(o.Workspace))
	// Permit filtering stok resources by component
	labels.SetLabel(ws, labels.WorkspaceComponent)

	ws.SetDebug(o.Debug)

	if isTTY {
		ws.Spec.AttachSpec.Handshake = true
		ws.Spec.AttachSpec.HandshakeTimeout = o.HandshakeTimeout.String()
	}

	return o.WorkspacesClient(o.Namespace).Create(ctx, ws, metav1.CreateOptions{})
}

func (o *NewOptions) createSecretIfMissing(ctx context.Context) error {
	// Shorter var name for readability
	name := o.WorkspaceSpec.SecretName

	// Check if it exists already
	_, err := o.SecretsClient(o.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
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
		if errors.IsNotFound(err) {
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
	// Set stok's common labels
	labels.SetCommonLabels(secret)
	// Permit filtering secrets by workspace
	labels.SetLabel(secret, labels.Workspace(o.Workspace))
	// Permit filtering stok resources by component
	labels.SetLabel(secret, labels.WorkspaceComponent)

	return o.SecretsClient(o.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (o *NewOptions) createServiceAccount(ctx context.Context, name string) (*corev1.ServiceAccount, error) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	// Set stok's common labels
	labels.SetCommonLabels(serviceAccount)
	// Permit filtering service accounts by workspace
	labels.SetLabel(serviceAccount, labels.Workspace(o.Workspace))
	// Permit filtering stok resources by component
	labels.SetLabel(serviceAccount, labels.WorkspaceComponent)

	return o.ServiceAccountsClient(o.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
}

// waitForContainer returns true once the runner container can be
// attached/stream to/from
func (o *NewOptions) waitForContainer(ctx context.Context, ws *v1alpha1.Workspace, attaching bool) (*corev1.Pod, error) {
	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: ws.PodName(), Namespace: ws.GetNamespace()}
	hdlr := handlers.ContainerReady(ws.PodName(), runner.ContainerName, true, attaching)
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, hdlr)
	return event.Object.(*corev1.Pod), err
}

func detectTTY(in interface{}) bool {
	if term.IsTerminal(in) {
		log.Debug("Detected TTY on stdin")
		return true
	} else {
		log.Debug("TTY not detected on stdin")
		return false
	}
}
