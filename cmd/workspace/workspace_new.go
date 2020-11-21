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
	stokerrors "github.com/leg100/stok/pkg/errors"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/runner"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"

	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/logstreamer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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

	ws, err := o.createWorkspace(ctx)
	if err != nil {
		return err
	}
	o.createdWorkspace = true
	log.Infof("Created workspace %s\n", o.name())

	// Monitor pod until it is running and ready
	pod, err := o.monitor(ctx, ws, isTTY)
	if err != nil {
		return err
	}

	// Monitor exit code; non-blocking
	exit := o.monitorExit(ctx)

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

func (o *NewOptions) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Workspace,
			Namespace: o.Namespace,
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-operator",

				// Unique name of instance within application
				"app.kubernetes.io/instance": o.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
		Spec: o.WorkspaceSpec,
	}

	ws.SetDebug(o.Debug)

	if term.IsTerminal(o.In) {
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
			Labels: map[string]string{
				// Name of the application
				"app":                    "stok",
				"app.kubernetes.io/name": "stok",

				// Name of higher-level application this app is part of
				"app.kubernetes.io/part-of": "stok",

				// The tool being used to manage the operation of an application
				"app.kubernetes.io/managed-by": "stok-cli",

				// Unique name of instance within application
				"app.kubernetes.io/instance": o.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
	}

	return o.SecretsClient(o.Namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (o *NewOptions) createServiceAccount(ctx context.Context, name string) (*corev1.ServiceAccount, error) {
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
				"app.kubernetes.io/instance": o.Workspace,

				// Current version of application
				"version":                   version.Version,
				"app.kubernetes.io/version": version.Version,

				// Component within architecture
				"component":                   "workspace",
				"app.kubernetes.io/component": "workspace",
			},
		},
	}

	return o.ServiceAccountsClient(o.Namespace).Create(ctx, serviceAccount, metav1.CreateOptions{})
}

// Return true if pod is both running and ready
func (o *NewOptions) monitor(ctx context.Context, ws *v1alpha1.Workspace, attaching bool) (*corev1.Pod, error) {
	// Current pod phase
	var phase corev1.PodPhase
	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: ws.PodName(), Namespace: ws.GetNamespace()}

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
		pod := event.Object.(*corev1.Pod)

		// ListWatcher field selector filters out other pods but the fake client doesn't implement the
		// field selector, so the following is necessary purely for testing purposes
		if pod.GetName() != ws.PodName() {
			return false, nil
		}

		if event.Type == watch.Deleted {
			return false, fmt.Errorf("workspace pod resource deleted")
		}

		if pod.Status.Phase != phase {
			log.Debugf("Pod phase shift: %s -> %s\n", pod.Status.Phase, phase)
			phase = pod.Status.Phase
		}

		switch phase {
		case corev1.PodRunning:
			return false, fmt.Errorf("workspace pod unexpectedly running")
		case corev1.PodSucceeded:
			return false, fmt.Errorf("workspace pod unexpectedly succeeded")
		case corev1.PodFailed:
			return false, fmt.Errorf(pod.Status.InitContainerStatuses[0].State.Terminated.Message)
		case corev1.PodPending:
			if len(pod.Status.InitContainerStatuses) > 0 {
				state := pod.Status.InitContainerStatuses[0].State
				if state.Running != nil {
					// Pod is both attachable and streamable
					return true, nil
				}
				if state.Terminated != nil && !attaching {
					// Pod is streamable (but not attachable)
					return true, nil
				}
			}
		}
		return false, nil
	})
	return event.Object.(*corev1.Pod), err
}

// Wait for the pod to complete and propagate its error, it has one. The error implements
// errors.ExitError if there is an error, which contains the non-zero exit code of the container.
// Non-blocking, the error is reported via the returned error channel.
func (o *NewOptions) monitorExit(ctx context.Context) chan error {
	var code int
	exit := make(chan error)
	go func() {
		lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: v1alpha1.WorkspacePodName(o.Workspace), Namespace: o.Namespace}
		_, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(event watch.Event) (bool, error) {
			pod := event.Object.(*corev1.Pod)

			// ListWatcher field selector filters out other pods but the fake client doesn't implement the
			// field selector, so the following is necessary purely for testing purposes
			if pod.GetName() != v1alpha1.WorkspacePodName(o.Workspace) {
				return false, nil
			}

			if len(pod.Status.InitContainerStatuses) > 0 {
				if pod.Status.InitContainerStatuses[0].State.Terminated != nil {
					code = int(pod.Status.InitContainerStatuses[0].State.Terminated.ExitCode)
					return true, nil
				}
			}
			return false, nil
		})

		if err != nil {
			exit <- fmt.Errorf("failed to retrieve exit code: %w", err)
		} else if code != 0 {
			exit <- stokerrors.NewExitError(code)
		} else {
			exit <- nil
		}
	}()
	return exit
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
