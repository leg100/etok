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
	"github.com/leg100/etok/pkg/repo"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/logstreamer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

const (
	defaultReconcileTimeout = 10 * time.Second
	defaultPodTimeout       = 60 * time.Second
	defaultReadyTimeout     = 60 * time.Second
	defaultCacheSize        = "1Gi"
)

var (
	errPodTimeout         = errors.New("timed out waiting for pod to be ready")
	errReconcileTimeout   = errors.New("timed out waiting for workspace to be reconciled")
	errReadyTimeout       = errors.New("timed out waiting for workspace to be ready")
	errWorkspaceNameArg   = errors.New("expected single argument providing the workspace name")
	errRepositoryNotFound = errors.New("repository not found: workspace path must be within a git repository")
)

type newOptions struct {
	*cmdutil.Factory

	*client.Client

	path        string
	namespace   string
	workspace   string
	kubeContext string

	// etok Workspace's workspaceSpec
	workspaceSpec v1alpha1.WorkspaceSpec

	// Timeout for resource to be reconciled (at least once)
	reconcileTimeout time.Duration

	// Timeout for workspace pod to be ready
	podTimeout time.Duration

	// Timeout for workspace ready condition to be true
	readyTimeout time.Duration

	// Disable default behaviour of deleting resources upon error
	disableResourceCleanup bool

	// Recall if resources are created so that if error occurs they can be
	// cleaned up
	createdWorkspace bool

	// For testing purposes set workspace status
	status *v1alpha1.WorkspaceStatus

	variables            map[string]string
	environmentVariables map[string]string

	etokenv *env.Env
}

func newCmd(f *cmdutil.Factory) (*cobra.Command, *newOptions) {
	o := &newOptions{
		Factory:   f,
		namespace: defaultNamespace,
	}
	cmd := &cobra.Command{
		Use:   "new <workspace>",
		Short: "Create a new etok workspace",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if len(args) != 1 {
				return errWorkspaceNameArg
			}

			o.workspace = args[0]

			// Ensure path is within a git repository
			_, err = repo.Open(o.path)
			if err != nil {
				return err
			}

			o.etokenv, err = env.New(o.namespace, o.workspace)
			if err != nil {
				return err
			}

			// Storage class default is nil not empty string (pflags doesn't
			// permit default of nil)
			if !flags.IsFlagPassed(cmd.Flags(), "storage-class") {
				o.workspaceSpec.Cache.StorageClass = nil
			}

			o.Client, err = f.Create(o.kubeContext)
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
	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)
	flags.AddDisableResourceCleanupFlag(cmd, &o.disableResourceCleanup)

	cmd.Flags().StringVar(&o.workspaceSpec.Cache.Size, "size", defaultCacheSize, "Size of PersistentVolume for cache")
	cmd.Flags().StringVar(&o.workspaceSpec.TerraformVersion, "terraform-version", "", "Override terraform version")
	cmd.Flags().BoolVarP(&o.workspaceSpec.Ephemeral, "ephemeral", "e", false, "Disable state backup (and restore)")

	// We want nil to be the default but it doesn't seem like pflags supports
	// that so use empty string and override later (see above)
	o.workspaceSpec.Cache.StorageClass = cmd.Flags().String("storage-class", "", "StorageClass of PersistentVolume for cache")

	cmd.Flags().DurationVar(&o.reconcileTimeout, "reconcile-timeout", defaultReconcileTimeout, "timeout for resource to be reconciled")
	cmd.Flags().DurationVar(&o.podTimeout, "pod-timeout", defaultPodTimeout, "timeout for pod to be ready")
	cmd.Flags().DurationVar(&o.readyTimeout, "ready-timeout", defaultReadyTimeout, "timeout for ready condition to report true")

	cmd.Flags().StringSliceVar(&o.workspaceSpec.PrivilegedCommands, "privileged-commands", []string{}, "Set privileged commands")

	cmd.Flags().StringToStringVar(&o.variables, "variables", map[string]string{}, "Set terraform variables")
	cmd.Flags().StringToStringVar(&o.environmentVariables, "environment-variables", map[string]string{}, "Set environment variables")

	return cmd, o
}

func (o *newOptions) run(ctx context.Context) error {
	ws, err := o.createWorkspace(ctx)
	if err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)

	// Wait for resource to have been successfully reconciled at least once
	// within the ReconcileTimeout (If we don't do this and the operator is
	// either not installed or malfunctioning then the user would be none the
	// wiser until the much longer PodTimeout had expired).
	g.Go(func() error {
		return o.waitForReconcile(gctx, ws)
	})

	// Wait for workspace to be ready
	g.Go(func() error {
		return o.waitForReady(gctx, ws)
	})

	// Monitor exit code; non-blocking
	exit := monitors.ExitMonitor(ctx, o.KubeClient, ws.PodName(), ws.Namespace, controllers.InstallerContainerName)

	// Wait for pod to be ready and start streaming logs from its installer
	// container
	g.Go(func() error {
		fmt.Fprintln(o.Out, "Waiting for workspace pod to be ready...")
		_, err := o.waitForContainer(gctx, ws)
		if err != nil {
			return err
		}

		return logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.namespace), ws.PodName(), controllers.InstallerContainerName)
	})

	// Wait for workspace to have been reconciled and for its pod container to
	// be ready
	if err := g.Wait(); err != nil {
		return err
	}

	if err := o.etokenv.Write(o.path); err != nil {
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
}

func (o *newOptions) createWorkspace(ctx context.Context) (*v1alpha1.Workspace, error) {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.workspace,
			Namespace: o.namespace,
		},
		Spec: o.workspaceSpec,
	}

	// Set etok's common labels
	labels.SetCommonLabels(ws)
	// Permit filtering secrets by workspace
	labels.SetLabel(ws, labels.Workspace(o.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(ws, labels.WorkspaceComponent)

	ws.Spec.Verbosity = o.Verbosity

	if o.status != nil {
		// For testing purposes seed workspace status
		ws.Status = *o.status
	}

	for k, v := range o.variables {
		ws.Spec.Variables = append(ws.Spec.Variables, &v1alpha1.Variable{Key: k, Value: v})
	}

	for k, v := range o.environmentVariables {
		ws.Spec.Variables = append(ws.Spec.Variables, &v1alpha1.Variable{Key: k, Value: v, EnvironmentVariable: true})
	}

	ws, err := o.WorkspacesClient(o.namespace).Create(ctx, ws, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	o.createdWorkspace = true
	fmt.Fprintf(o.Out, "Created workspace %s\n", klog.KObj(ws))

	return ws, nil
}

// waitForContainer returns true once the installer container can be streamed
// from
func (o *newOptions) waitForContainer(ctx context.Context, ws *v1alpha1.Workspace) (*corev1.Pod, error) {
	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: ws.PodName(), Namespace: ws.Namespace}
	hdlr := handlers.ContainerReady(ws.PodName(), controllers.InstallerContainerName, true, false)

	ctx, cancel := context.WithTimeout(ctx, o.podTimeout)
	defer cancel()

	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return nil, errPodTimeout
		}
		return nil, err
	}
	return event.Object.(*corev1.Pod), nil
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

// waitForReady waits for the ready condition to indicate it is ready.
func (o *newOptions) waitForReady(ctx context.Context, ws *v1alpha1.Workspace) error {
	lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: ws.Name, Namespace: ws.Namespace}
	hdlr := handlers.WorkspaceReady()

	ctx, cancel := context.WithTimeout(ctx, o.readyTimeout)
	defer cancel()

	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return errReadyTimeout
		}
		return err
	}
	return nil
}
