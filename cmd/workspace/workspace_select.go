package workspace

import (
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/spf13/cobra"
)

func SelectCmd(opts *app.Options) *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "select <[namespace/]workspace>",
		Short: "Select a stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			stokenv := env.WithOptionalNamespace(args[0])
			if err := stokenv.Write(path); err != nil {
				return err
			}
			fmt.Fprintf(opts.Out, "Current workspace now: %s\n", stokenv)
			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)

	return cmd
}
