package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
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
}

func (r *Runner) Run(ctx context.Context) error {
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

// Watch run/workspace until the 'wait' annotation has been cleared; this indicates that the client is in
// place to stream logs
func (r *Runner) sync(ctx context.Context) error {
	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	var lw cache.ListerWatcher
	var obj runtime.Object

	switch r.Kind {
	case "Run":
		lw = &k8s.RunListWatcher{Client: sc, Name: r.Name, Namespace: r.Namespace}
		obj = &v1alpha1.Run{}
	case "Workspace":
		lw = &k8s.WorkspaceListWatcher{Client: sc, Name: r.Name, Namespace: r.Namespace}
		obj = &v1alpha1.Workspace{}
	default:
		return fmt.Errorf("invalid kind: %s", r.Kind)
	}

	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	_, err = watchtools.UntilWithSync(ctx, lw, obj, nil, isSyncHandler)
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
