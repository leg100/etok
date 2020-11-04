package launcher

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/errors"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/pkg/logstreamer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	defaultTimeoutClient = 10 * time.Second
	defaultWorkspace     = "default"
)

type LauncherOptions struct {
	*cmdutil.Options

	args []string

	*client.Client

	Path        string
	Namespace   string
	Workspace   string
	KubeContext string
	RunName     string

	// Space delimited command to be run on pod
	Command string
	// Stok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec
	// Create a service acccount if it does not exist
	DisableCreateServiceAccount bool
	// Create a secret if it does not exist
	DisableCreateSecret bool
	// Timeout for runner to wait for magic string
	TimeoutClient time.Duration
	// Timeout for run pod to be running and ready
	TimeoutPod time.Duration
	// timeout waiting in workspace queue
	TimeoutQueue time.Duration `default:"1h"`
	// TODO: rename to timeout-pending (enqueue is too similar sounding to queue)
	// timeout waiting to be queued
	TimeoutEnqueue time.Duration `default:"10s"`
}

func (o *LauncherOptions) lookupEnvFile(cmd *cobra.Command) error {
	stokenv, err := env.ReadStokEnv(o.Path)
	if err != nil {
		// It's ok for envfile to not exist
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		// Override only if user has not set via flags
		if !isFlagPassed(cmd.Flags(), "namespace") {
			o.Namespace = stokenv.Namespace()
		}
		if !isFlagPassed(cmd.Flags(), "workspace") {
			o.Workspace = stokenv.Workspace()
		}
	}
	return nil
}

// Wrap shell args into a single command string
func wrapShellArgs(args []string) []string {
	if len(args) > 0 {
		return []string{"-c", strings.Join(args, " ")}
	} else {
		return []string{}
	}
}

// Check if user has passed a flag
func isFlagPassed(fs *pflag.FlagSet, name string) (found bool) {
	fs.Visit(func(f *pflag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

type podExitCodeMonitor struct {
	pod *corev1.Pod
}

func (pm *podExitCodeMonitor) handler(event watch.Event) (bool, error) {
	pod := event.Object.(*corev1.Pod)

	// ListWatcher field selector filters out other pods but the fake client doesn't implement the
	// field selector, so the following is necessary purely for testing purposes
	if pod.GetName() != pm.pod.GetName() {
		return false, nil
	}

	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("pod resource deleted")
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return true, nil
	default:
		return false, nil
	}
}
func (o *LauncherOptions) Run(ctx context.Context) error {
	isTTY := term.IsTerminal(o.In)

	// Tar up local config and deploy k8s resources
	run, err := o.deploy(ctx, isTTY)
	if err != nil {
		return err
	}

	// Monitor resources, wait until pod is running and ready
	pod, err := o.monitor(ctx, run)
	if err != nil {
		return err
	}

	// monitor exit code
	exitch := make(chan error)
	errch := make(chan error)
	lw := &k8s.PodListWatcher{Client: o.KubeClient, Name: pod.GetName(), Namespace: pod.GetNamespace()}
	go func() {
		event, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, (&podExitCodeMonitor{pod: pod}).handler)
		if err != nil {
			exitch <- fmt.Errorf("failed to retrieve exit code: %w", err)
		} else {
			pod := event.Object.(*corev1.Pod)
			exitch <- errors.NewExitError(
				int(pod.Status.ContainerStatuses[0].State.Terminated.ExitCode),
			)
		}
	}()

	go func() {
		if isTTY {
			errch <- o.AttachFunc(o.Out, *o.Config, pod, o.In.(*os.File), cmdutil.MagicString, cmdutil.ContainerName)
		} else {
			errch <- logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.Namespace), o.RunName, cmdutil.ContainerName)
		}
	}()

	// Exit code
	var exiterr error
	var completed, exited bool
	var timeout <-chan time.Time
	for {
		if completed && exited {
			return exiterr
		}
		select {
		case err := <-errch:
			if err != nil {
				// attach/streamer failure
				return err
			}
			completed = true
			// Start timer ticking to wait for exit code
			timeout = time.After(10 * time.Second)
		case exiterr = <-exitch:
			exited = true
		case <-timeout:
			return fmt.Errorf("timed out waiting for exit code handler to finish")
		}
	}
}

func (o *LauncherOptions) monitor(ctx context.Context, run *v1alpha1.Run) (*corev1.Pod, error) {
	var workspaceExists bool
	var pod *corev1.Pod
	errch := make(chan error)
	podch := make(chan *corev1.Pod)
	workspace := make(chan error)

	// Check workspace exists
	go func() {
		_, err := o.WorkspacesClient(o.Namespace).Get(ctx, o.Workspace, metav1.GetOptions{})
		workspace <- err
	}()

	// Non-blocking; watch workspace queue, check timeouts are not exceeded, and log run's queue position
	(&queueMonitor{
		run:            run,
		workspace:      o.Workspace,
		client:         o.StokClient,
		timeoutEnqueue: o.TimeoutEnqueue,
		timeoutQueue:   o.TimeoutQueue,
	}).monitor(ctx, errch)

	// Non-blocking; watch run, log status updates
	(&runMonitor{
		run:    run,
		client: o.StokClient,
	}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		run:    run,
		client: o.KubeClient,
	}).monitor(ctx, podch, errch)

	// Wait for pod to be ready and workspace confirmed to exist
	for {
		if pod != nil && workspaceExists {
			return pod, nil
		}
		select {
		case pod = <-podch:
			// nothing to be done
		case err := <-workspace:
			if err != nil {
				return nil, err
			}
			log.Debugf("confirmed workspace %s/%s exists\n", o.Namespace, o.Workspace)
			workspaceExists = true
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-errch:
			return nil, err
		}
	}
}

// Deploy configmap and run resources in parallel
func (o *LauncherOptions) deploy(ctx context.Context, isTTY bool) (run *v1alpha1.Run, err error) {
	g, ctx := errgroup.WithContext(ctx)

	// Compile tarball of terraform module, embed in configmap and deploy
	g.Go(func() error {
		tarball, err := archive.Create(o.Path)
		if err != nil {
			return err
		}

		// Construct and deploy ConfigMap resource
		return o.createConfigMap(ctx, tarball, o.RunName, v1alpha1.RunDefaultConfigMapKey)
	})

	// Construct and deploy command resource
	g.Go(func() error {
		run, err = o.createRun(ctx, o.RunName, o.RunName, isTTY)
		return err
	})

	return run, g.Wait()
}
