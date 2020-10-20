package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/log"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type Runner struct {
	*app.Options
}

func NewFromOpts(opts *app.Options) app.App {
	return &Runner{
		Options: opts,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	// Concurrently extract tarball
	if r.Tarball != "" {
		g.Go(func() error {
			_, err := archive.Extract(r.Tarball, r.Path)
			return err
		})
	}

	// Concurrently wait for client to release hold
	g.Go(func() error {
		return r.sync(gctx)
	})

	// Wait for concurrent tasks to complete
	if err := g.Wait(); err != nil {
		return err
	}

	return r.run(ctx, r.Out, os.Stderr)
}

// Watch run/workspace until the 'wait' annotation has been cleared; this indicates that the client is in
// place to stream logs
func (r *Runner) sync(ctx context.Context) error {
	var lw cache.ListerWatcher
	var obj runtime.Object

	switch r.Kind {
	case "Run":
		lw = &k8s.RunListWatcher{Client: r.StokClient(), Name: r.Name, Namespace: r.Namespace}
		obj = &v1alpha1.Run{}
	case "Workspace":
		lw = &k8s.WorkspaceListWatcher{Client: r.StokClient(), Name: r.Name, Namespace: r.Namespace}
		obj = &v1alpha1.Workspace{}
	default:
		return fmt.Errorf("invalid kind: %s", r.Kind)
	}

	ctx, cancel := context.WithTimeout(ctx, r.TimeoutClient)
	defer cancel()

	_, err := watchtools.UntilWithSync(ctx, lw, obj, nil, isSyncHandler)
	return err
}

// Run args, taking first arg as executable, and remainder as args to executable. Path sets the
// working directory of the executable; out and errout set stdout and stderr of executable.
func (r *Runner) run(ctx context.Context, out, errout io.Writer) error {
	return Run(ctx, r.Args, r.Path, out, errout)
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
