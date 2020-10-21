package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("show").
			WithShortHelp("Show current workspace").
			WithFlags(
				flags.Path,
			).
			WithExec(func(ctx context.Context, opts *app.Options) error {
				stokenv, err := env.ReadStokEnv(opts.Path)
				if err != nil {
					if os.IsNotExist(err) {
						// no .terraform/environment, so show defaults
						fmt.Fprintf(opts.Out, "%s/%s\n", opts.Namespace, opts.Workspace)
						return nil
					}
					return err
				}

				fmt.Fprintln(opts.Out, string(stokenv))
				return nil
			}))
}
