package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/util/slice"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type runnerCmd struct {
	Path    string
	Tarball string

	Name      string
	Namespace string
	Kind      string
	Timeout   time.Duration
	Context   string
	NoWait    bool

	factory k8s.FactoryInterface
	args    []string
	cmd     *cobra.Command
}

func newRunnerCmd(f k8s.FactoryInterface) *cobra.Command {
	runner := &runnerCmd{}

	cmd := &cobra.Command{
		// TODO: what is the syntax for stating at least one command must be provided?
		Use:           "runner [command (args)]",
		Short:         "Run the stok runner",
		Long:          "The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runner.doRunnerCmd(args); err != nil {
				return fmt.Errorf("runner: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&runner.Path, "path", ".", "Workspace config path")
	cmd.Flags().StringVar(&runner.Tarball, "tarball", "", "Extract specified tarball file to workspace path")

	cmd.Flags().BoolVar(&runner.NoWait, "no-wait", false, "Disable polling resource for client annotation")
	cmd.Flags().StringVar(&runner.Name, "name", "", "Name of command resource")
	cmd.Flags().StringVar(&runner.Namespace, "namespace", "default", "Namespace of command resource")
	cmd.Flags().StringVar(&runner.Kind, "kind", "", "Kind of command resource")
	cmd.Flags().StringVar(&runner.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")
	cmd.Flags().DurationVar(&runner.Timeout, "timeout", 10*time.Second, "Timeout on waiting for client to confirm readiness")

	runner.factory = f
	runner.cmd = cmd
	return runner.cmd
}

func (r *runnerCmd) doRunnerCmd(args []string) error {
	if err := r.validate(); err != nil {
		return err
	}

	r.args = args
	ctx := r.cmd.Context()
	g, gctx := errgroup.WithContext(ctx)

	// Concurrently extract tarball, if specified
	if r.Tarball != "" {
		g.Go(func() error {
			return archive.Extract(r.Tarball, r.Path)
		})
	}

	// Concurrently wait for client to release hold, if specified
	if !r.NoWait {
		g.Go(func() error {
			return r.handleSemaphore(gctx)
		})
	}

	// Wait for concurrent tasks to complete
	if err := g.Wait(); err != nil {
		return err
	}

	// Tasks succeeded; run command
	return r.run(ctx, os.Stdout, os.Stderr)
}

func (r *runnerCmd) validate() error {
	if r.Kind == "" {
		return fmt.Errorf("missing flag: --kind <kind>")
	}

	if !slice.ContainsString(append(command.CommandKinds, "Workspace"), r.Kind) {
		return fmt.Errorf("invalid kind: %s", r.Kind)
	}

	return nil
}

func (r *runnerCmd) handleSemaphore(ctx context.Context) error {
	config, err := r.factory.NewConfig(r.Context)
	if err != nil {
		return err
	}

	rc, err := r.factory.NewClient(config)
	if err != nil {
		return err
	}

	mgr, err := r.factory.NewManager(config, r.Namespace)
	if err != nil {
		return err
	}

	mgr.AddReporter(&RunnerReporter{
		Client:  rc,
		name:    r.Name,
		kind:    r.Kind,
		timeout: r.Timeout,
	})

	return mgr.Start(ctx)
}

// Run args, taking first arg as executable, and remainder as args to executable. Path sets the
// working directory of the executable; out and errout set stdout and stderr of executable.
func (r *runnerCmd) run(ctx context.Context, out, errout io.Writer) error {
	args := command.RunnerArgsForKind(r.Kind, r.args)

	log.WithFields(log.Fields{"command": args[0], "args": args[1:]}).Debug("running command")

	exe := exec.CommandContext(ctx, args[0], args[1:]...)
	exe.Dir = r.Path
	exe.Stdin = os.Stdin
	exe.Stdout = out
	exe.Stderr = errout

	return exe.Run()
}
