package cmd

import (
	"context"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/log"
)

func init() {
	workspaceCmd.AddChild(
		NewCmd("select <[namespace/]workspace>").
			WithShortHelp("Select a stok workspace").
			WithFlags(
				flags.Path,
			).
			WithOneArg().
			WithExec(func(ctx context.Context, opts *app.Options) error {
				stokenv := env.WithOptionalNamespace(opts.Args[0])
				if err := stokenv.Write(opts.Path); err != nil {
					return err
				}
				log.Infof("Current workspace now: %s\n", stokenv)
				return nil
			}))
}
