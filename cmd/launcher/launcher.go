package launcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/handlers"
	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/monitors"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	defaultWorkspace        = "default"
	defaultReconcileTimeout = 10 * time.Second
)

var (
	errNotAuthorised     = errors.New("you are not authorised")
	errEnqueueTimeout    = errors.New("timed out waiting for run to be enqueued")
	errWorkspaceNotFound = errors.New("workspace not found")
	errReconcileTimeout  = errors.New("timed out waiting for run to be reconciled")
)

// launcherOptions deploys a new Run. It monitors not only its progress, but
// that of its pod and its workspace too. It stream logs from the pod to the
// client, or, if a TTY is detected on the client, it attaches the client to the
// pod's TTY, permitting input/output. It then awaits the completion of the pod,
// reporting its container's exit code.
type launcherOptions struct {
	*cmdutil.Options

	args []string

	*client.Client

	path        string
	namespace   string
	workspace   string
	kubeContext string
	runName     string

	// command to be run on pod
	command string
	// etok Workspace's workspaceSpec
	workspaceSpec v1alpha1.WorkspaceSpec
	// Create a service acccount if it does not exist
	disableCreateServiceAccount bool
	// Create a secret if it does not exist
	disableCreateSecret bool

	// Disable default behaviour of deleting resources upon error
	disableResourceCleanup bool

	// Timeout for wait for handshake
	handshakeTimeout time.Duration
	// Timeout for run pod to be running and ready
	podTimeout time.Duration
	// timeout waiting to be queued
	enqueueTimeout time.Duration
	// Timeout for resource to be reconciled (at least once)
	reconcileTimeout time.Duration

	// Disable TTY detection
	disableTTY bool

	// Recall if resources are created so that if error occurs they can be cleaned up
	createdRun     bool
	createdArchive bool

	// For testing purposes toggle obj having been reconciled
	reconciled bool
}

func (o *launcherOptions) lookupEnvFile(cmd *cobra.Command) error {
	etokenv, err := env.Read(o.path)
	if err != nil {
		// It's ok for envfile to not exist
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !flags.IsFlagPassed(cmd.Flags(), "namespace") {
			o.namespace = etokenv.Namespace
		}
		if !flags.IsFlagPassed(cmd.Flags(), "workspace") {
			o.workspace = etokenv.Workspace
		}
	}
	return nil
}

func (o *launcherOptions) run(ctx context.Context) error {
	isTTY := !o.disableTTY && term.IsTerminal(o.In)

	// Tar up local config and deploy k8s resources
	run, err := o.deploy(ctx, isTTY)
	if err != nil {
		return err
	}

	// Watch and log run updates
	o.watchRun(ctx, run)

	if o.command != "plan" {
		// Only commands other than plan are queued - watch and log queue
		// updates
		o.watchQueue(ctx, run)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Wait for resource to have been successfully reconciled at least once
	// within the ReconcileTimeout (If we don't do this and the operator is
	// either not installed or malfunctioning then the user would be none the
	// wiser until the much longer PodTimeout had expired).
	g.Go(func() error {
		return o.waitForReconcile(gctx, run)
	})

	// Wait for pod and when ready send pod on chan
	podch := make(chan *corev1.Pod, 1)
	g.Go(func() error {
		return o.waitForPod(gctx, run, isTTY, podch)
	})

	if o.command != "plan" {
		// Only commands other than plan are queued - wait for run to be
		// enqueued
		g.Go(func() error {
			return o.waitForEnqueued(gctx, run)
		})
	}

	// In the meantime, check workspace exists
	ws, err := o.WorkspacesClient(o.namespace).Get(ctx, o.workspace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("%w: %s/%s", errWorkspaceNotFound, o.namespace, o.workspace)
	}
	// ...and approve run if command listed as privileged
	if ws.IsPrivilegedCommand(o.command) {
		if err := o.approveRun(ctx, ws, run); err != nil {
			return err
		}
	}

	// Carry on waiting for run to be enqueued (if not a plan) and for pod to be
	// ready
	if err := g.Wait(); err != nil {
		return err
	}

	// Receive ready pod
	pod := <-podch

	// Monitor exit code; non-blocking
	exit := monitors.ExitMonitor(ctx, o.KubeClient, pod.Name, pod.Namespace, globals.RunnerContainerName)

	// Connect to pod
	if isTTY {
		if err := o.AttachFunc(o.Out, *o.Config, pod, o.In.(*os.File), cmdutil.HandshakeString, globals.RunnerContainerName); err != nil {
			return err
		}
	} else {
		if err := logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.namespace), o.runName, globals.RunnerContainerName); err != nil {
			return err
		}
	}

	// Return container's exit code
	select {
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for exit code")
	case code := <-exit:
		return code
	}
}

// Non-blocking; watch pod; if tty then wait til pod is running (and then attach); if
// no tty then wait til pod is running or completed (and then stream logs from)
func (o *launcherOptions) waitForPod(ctx context.Context, run *v1alpha1.Run, isTTY bool, podch chan<- *corev1.Pod) error {
	ctx, cancel := context.WithTimeout(ctx, o.podTimeout)
	defer cancel()

	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: run.Name, Namespace: run.Namespace}
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, handlers.ContainerReady(run.Name, globals.RunnerContainerName, false, isTTY))
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			err = fmt.Errorf("timed out waiting for pod to be ready")
		}
		return err
	}
	klog.V(1).Info("pod ready")
	podch <- event.Object.(*corev1.Pod)
	return nil
}

// Wait until run has been enqueued onto workspace, or until timeout has been
// reached.
func (o *launcherOptions) waitForEnqueued(ctx context.Context, run *v1alpha1.Run) error {
	ctx, cancel := context.WithTimeout(ctx, o.enqueueTimeout)
	defer cancel()

	lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: o.workspace, Namespace: o.namespace}
	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, handlers.IsQueued(run.Name))
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			err = errEnqueueTimeout
		}
		return err
	}
	klog.V(1).Info("run enqueued within enqueue timeout")
	return nil
}

func (o *launcherOptions) watchRun(ctx context.Context, run *v1alpha1.Run) {
	go func() {
		lw := &k8s.RunListWatcher{Client: o.EtokClient, Name: run.Name, Namespace: run.Namespace}
		// Ignore errors
		// TODO: the current logger has no warning level. We should probably
		// upgrade the logger to something that does, and then log any error
		// here as a warning.
		_, _ = watchtools.UntilWithSync(ctx, lw, &v1alpha1.Run{}, nil, handlers.LogRunPhase())
	}()
}

func (o *launcherOptions) watchQueue(ctx context.Context, run *v1alpha1.Run) {
	go func() {
		lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: o.workspace, Namespace: o.namespace}
		// Ignore errors TODO: the current logger has no warning level. We
		// should probably upgrade the logger to something that does, and then
		// log any error here as a warning.
		_, _ = watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, handlers.LogQueuePosition(run.Name))
	}()
}

// Deploy configmap and run resources in parallel
func (o *launcherOptions) deploy(ctx context.Context, isTTY bool) (run *v1alpha1.Run, err error) {
	g, ctx := errgroup.WithContext(ctx)

	// Construct new archive
	arc, err := archive.NewArchive(o.path)
	if err != nil {
		return nil, err
	}

	// Add local module references to archive
	if err := arc.Walk(); err != nil {
		return nil, err
	}

	// Get relative path to root module within archive
	root, err := arc.RootPath()
	if err != nil {
		return nil, err
	}

	// Compile tarball of local terraform modules, embed in configmap and deploy
	g.Go(func() error {
		w := new(bytes.Buffer)
		meta, err := arc.Pack(w)
		if err != nil {
			return err
		}

		klog.V(1).Infof("slug created: %d files; %d (%d) bytes (compressed)\n", len(meta.Files), meta.Size, meta.CompressedSize)

		// Construct and deploy ConfigMap resource
		return o.createConfigMap(ctx, w.Bytes(), o.runName, v1alpha1.RunDefaultConfigMapKey)
	})

	// Construct and deploy command resource
	g.Go(func() error {
		run, err = o.createRun(ctx, o.runName, o.runName, isTTY, root)
		return err
	})

	return run, g.Wait()
}

func (o *launcherOptions) cleanup() {
	if o.createdRun {
		o.RunsClient(o.namespace).Delete(context.Background(), o.runName, metav1.DeleteOptions{})
	}
	if o.createdArchive {
		o.ConfigMapsClient(o.namespace).Delete(context.Background(), o.runName, metav1.DeleteOptions{})
	}
}

func (o *launcherOptions) approveRun(ctx context.Context, ws *v1alpha1.Workspace, run *v1alpha1.Run) error {
	klog.V(1).Infof("%s is a privileged command on workspace\n", o.command)
	annotations := ws.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[run.ApprovedAnnotationKey()] = "approved"
	ws.SetAnnotations(annotations)

	_, err := o.WorkspacesClient(o.namespace).Update(ctx, ws, metav1.UpdateOptions{})
	if err != nil {
		if kerrors.IsForbidden(err) {
			return fmt.Errorf("attempted to run privileged command %s: %w", o.command, errNotAuthorised)
		} else {
			return fmt.Errorf("failed to update workspace to approve privileged command: %w", err)
		}
	}
	klog.V(1).Info("successfully approved run with workspace")

	return nil
}

func (o *launcherOptions) createRun(ctx context.Context, name, configMapName string, isTTY bool, relPathToRoot string) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(o.namespace)
	run.SetName(name)

	// Set etok's common labels
	labels.SetCommonLabels(run)
	// Permit filtering runs by command
	labels.SetLabel(run, labels.Command(o.command))
	// Permit filtering runs by workspace
	labels.SetLabel(run, labels.Workspace(o.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(run, labels.RunComponent)

	run.Workspace = o.workspace

	run.Command = o.command
	run.Args = o.args
	run.ConfigMap = configMapName
	run.ConfigMapKey = v1alpha1.RunDefaultConfigMapKey
	run.ConfigMapPath = relPathToRoot

	run.Verbosity = o.Verbosity

	// For testing purposes mimic obj having been reconciled
	run.Reconciled = o.reconciled

	if isTTY {
		run.AttachSpec.Handshake = true
		run.AttachSpec.HandshakeTimeout = o.handshakeTimeout.String()
	}

	run, err := o.RunsClient(o.namespace).Create(ctx, run, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	o.createdRun = true
	klog.V(1).Infof("created run %s\n", klog.KObj(run))

	return run, nil
}

func (o *launcherOptions) createConfigMap(ctx context.Context, tarball []byte, name, keyName string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.namespace,
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering archives by command
	labels.SetLabel(configMap, labels.Command(o.command))
	// Permit filtering archives by workspace
	labels.SetLabel(configMap, labels.Workspace(o.workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(configMap, labels.RunComponent)

	_, err := o.ConfigMapsClient(o.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	o.createdArchive = true
	klog.V(1).Infof("created config map %s\n", klog.KObj(configMap))

	return nil
}

// waitForReconcile waits for the workspace resource to be reconciled.
func (o *launcherOptions) waitForReconcile(ctx context.Context, run *v1alpha1.Run) error {
	lw := &k8s.RunListWatcher{Client: o.EtokClient, Name: run.Name, Namespace: run.Namespace}
	hdlr := handlers.Reconciled(run)

	ctx, cancel := context.WithTimeout(ctx, o.reconcileTimeout)
	defer cancel()

	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Run{}, nil, hdlr)
	if err != nil {
		if errors.Is(err, wait.ErrWaitTimeout) {
			return errReconcileTimeout
		}
		return err
	}
	return nil
}
