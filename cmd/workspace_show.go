package cmd

import (
	"context"
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/options"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("show").
			WithShortUsage("show").
			WithShortHelp("Show current workspace").
			WithFlags(
				flags.Path,
			).
			WithOneArg().
			WithExec(func(ctx context.Context, opts *options.StokOptions) error {
				stokenv, err := env.ReadStokEnv(opts.Path)
				if err != nil {
					return err
				}

				fmt.Fprintln(opts.Out, string(stokenv))

				return nil
			}))
}
