package launcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	etokerrors "github.com/leg100/etok/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watchtools "k8s.io/client-go/tools/watch"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/attacher"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/handlers"
	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/monitors"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	DefaultWorkspace        = "default"
	DefaultNamespace        = "default"
	DefaultReconcileTimeout = 10 * time.Second
	DefaultPodTimeout       = time.Hour
)

var (
	errNotAuthorised     = errors.New("you are not authorised")
	errWorkspaceNotFound = errors.New("workspace not found")
	errWorkspaceNotReady = errors.New("workspace not ready")
	errReconcileTimeout  = errors.New("timed out waiting for run to be reconciled")
)

// LauncherOptions deploys a new Run. It monitors not only its progress, but
// that of its pod and its workspace too. It stream logs from the pod to the
// client, or, if a TTY is detected on the client, it attaches the client to the
// pod's TTY, permitting input/output. It then awaits the completion of the pod,
// reporting its container's exit code.
type LauncherOptions struct {
	Args []string

	*client.Client

	Path        string
	Namespace   string
	Workspace   string
	KubeContext string
	RunName     string

	*cmdutil.IOStreams

	Verbosity int

	// Function to attach to a pod's TTY
	attacher.AttachFunc

	// Function to get a pod's logs stream
	logstreamer.GetLogsFunc

	// Command to be run on pod
	Command string
	// etok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec

	// Disable default behaviour of deleting resources upon error
	DisableResourceCleanup bool

	// Timeout for wait for handshake
	HandshakeTimeout time.Duration
	// Timeout for run pod to be running and ready
	PodTimeout time.Duration
	// Timeout for resource to be reconciled (at least once)
	ReconcileTimeout time.Duration

	// Disable TTY detection
	DisableTTY bool

	// For testing purposes set run Status
	Status *v1alpha1.RunStatus
}

type Launcher struct {
	*LauncherOptions

	// Recall if resources are created so that if error occurs they can be cleaned up
	createdRun     bool
	createdArchive bool

	// attach toggles whether pod will be attached to (true), or streamed from
	// (false)
	attach bool
}

// NewLauncher constructs Launcher, validating required options, setting
// defaults on optional
func NewLauncher(opts *LauncherOptions) (*Launcher, error) {
	l := &Launcher{LauncherOptions: opts}

	if opts.Namespace == "" {
		opts.Namespace = DefaultNamespace
	}
	if opts.Workspace == "" {
		opts.Workspace = DefaultWorkspace
	}
	if opts.ReconcileTimeout == 0 {
		opts.ReconcileTimeout = DefaultReconcileTimeout
	}
	if opts.PodTimeout == 0 {
		opts.PodTimeout = DefaultPodTimeout
	}

	if opts.IOStreams == nil {
		opts.IOStreams = &cmdutil.IOStreams{
			Out:    os.Stdout,
			In:     os.Stdin,
			ErrOut: os.Stderr,
		}
	}

	// Toggle whether to attach to pod's TTY
	l.attach = !l.DisableTTY && term.IsTerminal(l.In)

	if l.attach && opts.AttachFunc == nil {
		opts.AttachFunc = attacher.Attach
	}
	if !l.attach && opts.GetLogsFunc == nil {
		opts.GetLogsFunc = logstreamer.GetLogs
	}

	return l, nil
}

func (l *Launcher) Launch(ctx context.Context) error {
	if err := l.doLaunch(ctx); err != nil {
		// Cleanup resources upon error. An exit code error means the runner ran
		// successfully but the program it executed failed with a non-zero exit
		// code. In this case, resources are not cleaned up.
		var exit etokerrors.ExitError
		if !errors.As(err, &exit) {
			if !l.DisableResourceCleanup {
				l.Cleanup()
			}
		}
		return err
	}

	return nil
}

func (l *Launcher) doLaunch(ctx context.Context) error {
	// Tar up local config and deploy k8s resources
	run, err := l.deploy(ctx)
	if err != nil {
		return err
	}

	if IsQueueable(l.Command) {
		// Watch and log queue updates
		l.watchQueue(ctx, run)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Wait for resource to have been successfully reconciled at least once
	// within the ReconcileTimeout (If we don't do this and the operator is
	// either not installed or malfunctioning then the user would be none the
	// wiser until the much longer PodTimeout had expired).
	g.Go(func() error {
		return l.waitForReconcile(gctx, run)
	})

	// Wait for run to indicate pod is running
	g.Go(func() error {
		return l.watchRun(gctx, run)
	})

	// Check workspace exists and is healthy
	if err := l.checkWorkspace(ctx, run); err != nil {
		return err
	}

	// Carry on waiting for run to indicate pod is ready
	if err := g.Wait(); err != nil {
		return err
	}

	// Watch the run for the container's exit code. Non-blocking.
	exit := monitors.RunExitMonitor(ctx, l.EtokClient, l.Namespace, l.RunName)

	// Connect to pod
	if l.attach {
		if err := l.AttachFunc(l.Out, *l.Config, l.Namespace, l.RunName, l.In.(*os.File), cmdutil.HandshakeString, globals.RunnerContainerName); err != nil {
			return err
		}
	} else {
		if err := logstreamer.Stream(ctx, l.GetLogsFunc, l.Out, l.PodsClient(l.Namespace), l.RunName, globals.RunnerContainerName); err != nil {
			return err
		}
	}

	// Await container's exit code
	select {
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timed out waiting for exit code")
	case code := <-exit:
		if code != nil {
			return code
		}
	}

	if UpdatesLockFile(l.Command) {
		// Some commands (e.g. terraform init) update the lock file,
		// .terraform.lock.hcl, and it's recommended that this be committed to
		// version control. So the runner copies it to a config map, and it is
		// here that that config map is retrieved.
		lock, err := l.ConfigMapsClient(l.Namespace).Get(ctx, v1alpha1.RunLockFileConfigMapName(run.Name), metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Write lock file to user's disk
		lockFilePath := filepath.Join(l.Path, globals.LockFile)
		if err := ioutil.WriteFile(lockFilePath, lock.BinaryData[globals.LockFile], 0644); err != nil {
			return err
		}

		klog.V(1).Infof("Written %s", lockFilePath)
	}

	return nil
}

func (l *Launcher) Cleanup() {
	if l.createdRun {
		l.RunsClient(l.Namespace).Delete(context.Background(), l.RunName, metav1.DeleteOptions{})
	}
	if l.createdArchive {
		l.ConfigMapsClient(l.Namespace).Delete(context.Background(), l.RunName, metav1.DeleteOptions{})
	}
}

func (l *Launcher) approveRun(ctx context.Context, ws *v1alpha1.Workspace, run *v1alpha1.Run) error {
	klog.V(1).Infof("%s is a privileged command on workspace\n", l.Command)
	annotations := ws.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[run.ApprovedAnnotationKey()] = "approved"
	ws.SetAnnotations(annotations)

	_, err := l.WorkspacesClient(l.Namespace).Update(ctx, ws, metav1.UpdateOptions{})
	if err != nil {
		if kerrors.IsForbidden(err) {
			return fmt.Errorf("attempted to run privileged command %s: %w", l.Command, errNotAuthorised)
		} else {
			return fmt.Errorf("failed to update workspace to approve privileged command: %w", err)
		}
	}
	klog.V(1).Info("successfully approved run with workspace")

	return nil
}

func (l *Launcher) watchRun(ctx context.Context, run *v1alpha1.Run) error {
	lw := &k8s.RunListWatcher{Client: l.EtokClient, Name: run.Name, Namespace: run.Namespace}
	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Run{}, nil, handlers.RunConnectable(run.Name, l.attach))
	return err
}

func (l *Launcher) watchQueue(ctx context.Context, run *v1alpha1.Run) {
	go func() {
		lw := &k8s.WorkspaceListWatcher{Client: l.EtokClient, Name: l.Workspace, Namespace: l.Namespace}
		// Ignore errors TODO: the current logger has no warning level. We
		// should probably upgrade the logger to something that does, and then
		// log any error here as a warning.
		_, _ = watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, handlers.LogQueuePosition(run.Name))
	}()
}

func (l *Launcher) checkWorkspace(ctx context.Context, run *v1alpha1.Run) error {
	ws, err := l.WorkspacesClient(l.Namespace).Get(ctx, l.Workspace, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return fmt.Errorf("%w: %s/%s", errWorkspaceNotFound, l.Namespace, l.Workspace)
	}
	if err != nil {
		return err
	}

	// ...ensure workspace is ready
	workspaceReady := meta.FindStatusCondition(ws.Status.Conditions, v1alpha1.WorkspaceReadyCondition)
	if workspaceReady == nil {
		return fmt.Errorf("%w: %s: ready condition not found", errWorkspaceNotReady, klog.KObj(ws))
	}
	if workspaceReady.Status != metav1.ConditionTrue {
		return fmt.Errorf("%w: %s: %s", errWorkspaceNotReady, klog.KObj(ws), workspaceReady.Message)
	}

	// ...approve run if command listed as privileged
	if ws.IsPrivilegedCommand(l.Command) {
		if err := l.approveRun(ctx, ws, run); err != nil {
			return err
		}
	}

	return nil
}

// Deploy configmap and run resources in parallel
func (l *Launcher) deploy(ctx context.Context) (run *v1alpha1.Run, err error) {
	g, ctx := errgroup.WithContext(ctx)

	// Construct new archive
	arc, err := archive.NewArchive(l.Path)
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
		return l.createConfigMap(ctx, w.Bytes(), l.RunName, v1alpha1.RunDefaultConfigMapKey)
	})

	// Construct and deploy command resource
	g.Go(func() error {
		run, err = l.createRun(ctx, l.RunName, l.RunName, root)
		return err
	})

	return run, g.Wait()
}

func (l *Launcher) createRun(ctx context.Context, name, configMapName string, relPathToRoot string) (*v1alpha1.Run, error) {
	run := &v1alpha1.Run{}
	run.SetNamespace(l.Namespace)
	run.SetName(name)

	// Set etok's common labels
	labels.SetCommonLabels(run)
	// Permit filtering runs by command
	labels.SetLabel(run, labels.Command(l.Command))
	// Permit filtering runs by workspace
	labels.SetLabel(run, labels.Workspace(l.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(run, labels.RunComponent)

	run.Workspace = l.Workspace

	run.Command = l.Command
	run.Args = l.Args
	run.ConfigMap = configMapName
	run.ConfigMapKey = v1alpha1.RunDefaultConfigMapKey
	run.ConfigMapPath = relPathToRoot

	run.Verbosity = l.Verbosity

	if l.Status != nil {
		// For testing purposes seed status
		run.RunStatus = *l.Status
	}

	if l.attach {
		run.AttachSpec.Handshake = true
		run.AttachSpec.HandshakeTimeout = l.HandshakeTimeout.String()
	}

	run, err := l.RunsClient(l.Namespace).Create(ctx, run, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	l.createdRun = true
	klog.V(1).Infof("created run %s\n", klog.KObj(run))

	return run, nil
}

func (l *Launcher) createConfigMap(ctx context.Context, tarball []byte, name, keyName string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: l.Namespace,
		},
		BinaryData: map[string][]byte{
			keyName: tarball,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering archives by command
	labels.SetLabel(configMap, labels.Command(l.Command))
	// Permit filtering archives by workspace
	labels.SetLabel(configMap, labels.Workspace(l.Workspace))
	// Permit filtering etok resources by component
	labels.SetLabel(configMap, labels.RunComponent)

	_, err := l.ConfigMapsClient(l.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	l.createdArchive = true
	klog.V(1).Infof("created config map %s\n", klog.KObj(configMap))

	return nil
}

func (l *Launcher) waitForReconcile(ctx context.Context, run *v1alpha1.Run) error {
	lw := &k8s.RunListWatcher{Client: l.EtokClient, Name: run.Name, Namespace: run.Namespace}
	hdlr := handlers.Reconciled(run)

	ctx, cancel := context.WithTimeout(ctx, l.ReconcileTimeout)
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
