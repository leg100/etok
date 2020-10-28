package launcher

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/logstreamer"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	defaultTimeoutClient = 10 * time.Second
	defaultWorkspace     = "default"
)

type LauncherOptions struct {
	*app.Options

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

	debug bool
}

func LauncherCmds(opts *app.Options) (cmds []*cobra.Command) {
	for k, v := range run.TerraformCommandMap {
		if len(v) > 0 {
			// Terraform 'family' of commands, i.e. terraform show mv|rm|pull|push|show
			parent := &cobra.Command{
				Use:   k,
				Short: fmt.Sprintf("terraform %s family of commands", k),
			}
			for _, child := range v {
				cmd, _ := LauncherCmd(opts, child)
				parent.AddCommand(cmd)
			}
		} else {
			cmd, _ := LauncherCmd(opts, k)
			cmds = append(cmds, cmd)
		}
	}
	return cmds
}

func LauncherCmd(opts *app.Options, tfcmd string) (*cobra.Command, *LauncherOptions) {
	o := &LauncherOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [flags] -- [%s args]", tfcmd, tfcmd),
		Short: launcherShortHelp(tfcmd),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.Command = tfcmd

			if tfcmd == "sh" {
				// Wrap shell args into a single command string
				o.args = wrapShellArgs(args)
			} else {
				o.args = args
			}

			o.RunName = fmt.Sprintf("run-%s", util.GenerateRandomString(5))

			o.Client, err = opts.Create(o.KubeContext)
			if err != nil {
				return err
			}

			stokenv, err := env.ReadStokEnv(o.Path)
			if err != nil {
				if os.IsNotExist(err) {
					// Fallback to values set from flags, whether user set or their defaults
					return nil
				}
				return err
			}

			if !isFlagPassed(cmd.Flags(), "namespace") {
				o.Namespace = stokenv.Namespace()
			}

			if !isFlagPassed(cmd.Flags(), "workspace") {
				o.Workspace = stokenv.Workspace()
			}

			return o.Run(cmd.Context())
		},
	}

	flags.AddPathFlag(cmd, &o.Path)
	flags.AddNamespaceFlag(cmd, &o.Namespace)
	flags.AddKubeContextFlag(cmd, &o.KubeContext)

	cmd.Flags().DurationVar(&o.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
	cmd.Flags().DurationVar(&o.TimeoutClient, "timeout-client", defaultTimeoutClient, "timeout for client to signal readiness")
	cmd.Flags().DurationVar(&o.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
	cmd.Flags().DurationVar(&o.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")
	cmd.Flags().StringVar(&o.Workspace, "workspace", defaultWorkspace, "Stok workspace")

	return cmd, o
}

func launcherShortHelp(tfcmd string) string {
	if tfcmd == "sh" {
		return "Run shell commands in workspace"
	} else {
		return fmt.Sprintf("Run terraform %s", tfcmd)
	}
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

func (o *LauncherOptions) Run(ctx context.Context) error {
	isTTY := term.IsTerminal(o.In)

	// Tar up local config and deploy k8s resources
	run, err := o.deploy(ctx, isTTY)
	if err != nil {
		return err
	}

	// Monitor resources, wait until pod is running and ready
	if err := o.monitor(ctx, run); err != nil {
		return err
	}

	if isTTY {
		pod, err := o.PodsClient(o.Namespace).Get(ctx, o.RunName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("getting pod %s/%s: %w", o.Namespace, o.RunName, err)
		}
		return o.AttachFunc(o.Out, *o.Config, pod, o.In.(*os.File), app.MagicString, app.ContainerName)
	} else {
		return logstreamer.Stream(ctx, o.GetLogsFunc, o.Out, o.PodsClient(o.Namespace), o.RunName, app.ContainerName)
	}
}

func (o *LauncherOptions) monitor(ctx context.Context, run *v1alpha1.Run) error {
	errch := make(chan error)
	ready := make(chan struct{})

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
