package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"

	"github.com/apex/log"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/pkg/apps"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/options"
	"github.com/leg100/stok/version"
	"k8s.io/client-go/kubernetes"
)

var (
	app  apps.App
	root = NewCmd("root")
)

func init() {
	root.WithShortUsage("stok <subcommand> [flags] [<arg>...]").
		WithLongHelp("Supercharge terraform on kubernetes").
		WithFlags(
			func(fs *flag.FlagSet, opts *options.StokOptions) {
				fs.BoolVar(&opts.Version, "v", false, "print version")
			},
		).
		WithExec(func(ctx context.Context, opts *options.StokOptions) error {
			if opts.Version {
				fmt.Fprintf(opts.Out, "stok version %s\t%s\n", version.Version, version.Commit)
				return nil
			}

			return flag.ErrHelp
		})
}

func ExecWithExitCode(ctx context.Context, args []string, out, errout io.Writer) (int, error) {
	if err := Exec(ctx, args, out, errout); err != nil {
		return unwrapExitCode(err), err
	}
	return 0, nil
}

func Exec(ctx context.Context, args []string, out, errout io.Writer) error {
	log.SetHandler(cli.New(out, errout))

	opts := &options.StokOptions{Out: out, ErrOut: errout}
	cmd := root.Build(opts, clientCreator)

	if err := cmd.ParseAndRun(ctx, args); err != nil {
		return err
	}

	if opts.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug logging enabled")
	}

	if app != nil {
		return app.Run(ctx)
	}

	return nil
}

func clientCreator(kubeCtx string) (stokclient.Interface, kubernetes.Interface, error) {
	sc, err := k8s.StokClient()
	if err != nil {
		return nil, nil, err
	}

	kc, err := k8s.KubeClient()
	if err != nil {
		return nil, nil, err
	}
	return sc, kc, nil
}

// Unwrap exit code from error message
func unwrapExitCode(err error) int {
	var exiterr *exec.ExitError
	if errors.As(err, &exiterr) {
		return exiterr.ExitCode()
	}
	return 1
}
