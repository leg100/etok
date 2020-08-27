package runner

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
	"golang.org/x/sync/errgroup"
)

type Runner struct {
	Path    string
	Tarball string

	Name      string
	Namespace string
	Kind      string
	Timeout   time.Duration
	Context   string
	NoWait    bool

	Args []string

	Factory k8s.FactoryInterface
}

func (r *Runner) validate() error {
	if r.Kind == "" {
		return fmt.Errorf("missing flag: --kind <kind>")
	}

	if !slice.ContainsString(append(command.CommandKinds, "Workspace"), r.Kind) {
		return fmt.Errorf("invalid kind: %s", r.Kind)
	}

	return nil
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)

	// Concurrently extract tarball, if specified
	if r.Tarball != "" {
		g.Go(func() error {
			_, err := archive.Extract(r.Tarball, r.Path)
			return err
		})
	}

	// Concurrently wait for client to release hold, if specified
	if !r.NoWait {
		g.Go(func() error {
			return r.sync(gctx)
		})
	}

	// Wait for concurrent tasks to complete
	if err := g.Wait(); err != nil {
		return err
	}

	return r.run(ctx, os.Stdout, os.Stderr)
}

func (r *Runner) sync(ctx context.Context) error {
	// TODO: have NewConfig take ctx parameter
	config, err := r.Factory.NewConfig(r.Context)
	if err != nil {
		return err
	}

	rc, err := r.Factory.NewClient(config)
	if err != nil {
		return err
	}

	mgr, err := r.Factory.NewManager(config, r.Namespace)
	if err != nil {
		return err
	}

	mgr.AddReporter(&reporter{
		Client:  rc,
		name:    r.Name,
		kind:    r.Kind,
		timeout: r.Timeout,
	})

	return mgr.Start(ctx)
}

// Run args, taking first arg as executable, and remainder as args to executable. Path sets the
// working directory of the executable; out and errout set stdout and stderr of executable.
func (r *Runner) run(ctx context.Context, out, errout io.Writer) error {
	args := command.RunnerArgsForKind(r.Kind, r.Args)
	return Run(ctx, args, r.Path, out, errout)
}

// Synchronously run command, taking first arg of args as executable, and remainder as arguments.
func Run(ctx context.Context, args []string, path string, out, errout io.Writer) error {
	log.Debugf("running command `%v`", args)

	exe := exec.CommandContext(ctx, args[0], args[1:]...)
	exe.Dir = path
	exe.Stdin = os.Stdin
	exe.Stdout = out
	exe.Stderr = errout

	return exe.Run()
}
