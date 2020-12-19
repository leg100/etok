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
	defaultWorkspace = "default/default"
)

var (
	errNotAuthorised     = errors.New("you are not authorised")
	errEnqueueTimeout    = errors.New("timed out waiting for run to be enqueued")
	errWorkspaceNotFound = errors.New("workspace not found")
)

// LauncherOptions deploys a new Run. It monitors not only its progress, but
// that of its pod and its workspace too. It stream logs from the pod to the
// client, or, if a TTY is detected on the client, it attaches the client to the
// pod's TTY, permitting input/output. It then awaits the completion of the pod,
// reporting its container's exit code.
type LauncherOptions struct {
	*cmdutil.Options

	args []string

	*client.Client

	Path        string
	Namespace   string
	Workspace   string
	KubeContext string
	RunName     string

	// Command to be run on pod
	Command string
	// etok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec
	// Create a service acccount if it does not exist
	DisableCreateServiceAccount bool
	// Create a secret if it does not exist
	DisableCreateSecret bool

	// Disable default behaviour of deleting resources upon error
	DisableResourceCleanup bool

	// Timeout for wait for handshake
	HandshakeTimeout time.Duration
	// Timeout for run pod to be running and ready
	PodTimeout time.Duration
	// timeout waiting to be queued
	EnqueueTimeout time.Duration

	// Disable TTY detection
	DisableTTY bool

	// Recall if resources are created so that if error occurs they can be cleaned up
	createdRun     bool
	createdArchive bool
}

func (o *LauncherOptions) lookupEnvFile(cmd *cobra.Command) error {
	etokenv, err := env.ReadEtokEnv(o.Path)
	if err != nil {
		// It's ok for envfile to not exist
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !flags.IsFlagPassed(cmd.Flags(), "workspace") {
			o.Namespace = etokenv.Namespace()
			o.Workspace = etokenv.Workspace()
		}
	}
	return nil
}

func (o *LauncherOptions) Run(ctx context.Context) error {
	isTTY := !o.DisableTTY && term.IsTerminal(o.In)

	// Tar up local config and deploy k8s resources
	run, err := o.deploy(ctx, isTTY)
	if err != nil {
		return err
	}

	// Watch and log run updates
	o.watchRun(ctx, run)

	if o.Command != "plan" {
		// Only commands other than plan are queued - watch and log queue
		// updates
		o.watchQueue(ctx, run)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Wait for pod and when ready send pod on chan
	podch := make(chan *corev1.Pod, 1)
	g.Go(func() error {
		return o.waitForPod(gctx, run, isTTY, podch)
	})

	if o.Command != "plan" {
		// Only commands other than plan are queued - wait for run to be
		// enqueued
		g.Go(func() error {
			return o.waitForEnqueued(ctx, run)
		})
	}

	// In the meantime, check workspace exists
	ws, err := o.WorkspacesClient(o.Namespace).Get(ctx, o.Workspace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("%w: %s/%s", errWorkspaceNotFound, o.Namespace, o.Workspace)
	}
	// ...and approve run if command listed as privileged
	if ws.IsPrivilegedCommand(o.Command) {
		if err := o.ApproveRun(ctx, ws, run); err != nil {
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
		if err := logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.Namespace), o.RunName, globals.RunnerContainerName); err != nil {
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
func (o *LauncherOptions) waitForPod(ctx context.Context, run *v1alpha1.Run, isTTY bool, podch chan<- *corev1.Pod) error {
	ctx, cancel := context.WithTimeout(ctx, o.PodTimeout)
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
func (o *LauncherOptions) waitForEnqueued(ctx context.Context, run *v1alpha1.Run) error {
	ctx, cancel := context.WithTimeout(ctx, o.EnqueueTimeout)
	defer cancel()

	lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: o.Workspace, Namespace: o.Namespace}
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

func (o *LauncherOptions) watchRun(ctx context.Context, run *v1alpha1.Run) {
	go func() {
		lw := &k8s.RunListWatcher{Client: o.EtokClient, Name: run.Name, Namespace: run.Namespace}
		// Ignore errors
		// TODO: the current logger has no warning level. We should probably
		// upgrade the logger to something that does, and then log any error
		// here as a warning.
		_, _ = watchtools.UntilWithSync(ctx, lw, &v1alpha1.Run{}, nil, handlers.LogRunPhase())
	}()
}

func (o *LauncherOptions) watchQueue(ctx context.Context, run *v1alpha1.Run) {
	go func() {
		lw := &k8s.WorkspaceListWatcher{Client: o.EtokClient, Name: o.Workspace, Namespace: o.Namespace}
		// Ignore errors TODO: the current logger has no warning level. We
		// should probably upgrade the logger to something that does, and then
		// log any error here as a warning.
		_, _ = watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, handlers.LogQueuePosition(run.Name))
	}()
}

// Deploy configmap and run resources in parallel
func (o *LauncherOptions) deploy(ctx context.Context, isTTY bool) (run *v1alpha1.Run, err error) {
	g, ctx := errgroup.WithContext(ctx)

	// Construct new archive
	arc, err := archive.NewArchive(o.Path)
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
		return o.createConfigMap(ctx, w.Bytes(), o.RunName, v1alpha1.RunDefaultConfigMapKey)
	})

	// Construct and deploy command resource
	g.Go(func() error {
		run, err = o.createRun(ctx, o.RunName, o.RunName, isTTY, root)
		return err
	})

	return run, g.Wait()
}

func (o *LauncherOptions) cleanup() {
	if o.createdRun {
		o.RunsClient(o.Namespace).Delete(context.Background(), o.RunName, metav1.DeleteOptions{})
	}
	if o.createdArchive {
		o.ConfigMapsClient(o.Namespace).Delete(context.Background(), o.RunName, metav1.DeleteOptions{})
	}
}

func (o *LauncherOptions) ApproveRun(ctx context.Context, ws *v1alpha1.Workspace, run *v1alpha1.Run) error {
	klog.V(1).Infof("%s is a privileged command on workspace\n", o.Command)
	annotations := ws.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[run.ApprovedAnnotationKey()] = "approved"
	ws.SetAnnotations(annotations)

	_, err := o.WorkspacesClient(o.Namespace).Update(ctx, ws, metav1.UpdateOptions{})
	if err != nil {
		if kerrors.IsForbidden(err) {
			return fmt.Errorf("attempted to run privileged command %s: %w", o.Command, errNotAuthorised)
		} else {
			return fmt.Errorf("failed to update workspace to approve privileged command: %w", err)
		}
	}
	klog.V(1).Info("successfully approved run with workspace")

	return nil
}

func (o *LauncherOptions) createRun(ctx context.Context, name, configMapName string, isTTY bool, relPathToRoot string) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(o.Namespace)
	run.SetName(name)

	// Set etok's common labels
	labels.SetCommonLabels(run)
	// Permit filtering runs by command
	labels.SetLabel(run, labels.Command(o.Command))
	// Permit filtering runs by workspace
	labels.SetLabel(run, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(run, labels.RunComponent)

	run.Workspace = o.Workspace

	run.Command = o.Command
	run.Args = o.args
	run.ConfigMap = configMapName
	run.ConfigMapKey = v1alpha1.RunDefaultConfigMapKey
	run.ConfigMapPath = relPathToRoot

	run.Verbosity = o.Verbosity

	if isTTY {
		run.AttachSpec.Handshake = true
		run.AttachSpec.HandshakeTimeout = o.HandshakeTimeout.String()
	}

	run, err := o.RunsClient(o.Namespace).Create(ctx, run, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	o.createdRun = true
	klog.V(1).Infof("created run %s\n", klog.KObj(run))

	return run, nil
}

func (o *LauncherOptions) createConfigMap(ctx context.Context, tarball []byte, name, keyName string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.Namespace,
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering archives by command
	labels.SetLabel(configMap, labels.Command(o.Command))
	// Permit filtering archives by workspace
	labels.SetLabel(configMap, labels.Workspace(o.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(configMap, labels.RunComponent)

	_, err := o.ConfigMapsClient(o.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	o.createdArchive = true
	klog.V(1).Infof("created config map %s\n", klog.KObj(configMap))

	return nil
}
