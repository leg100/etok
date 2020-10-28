package workspace

import (
	"fmt"
	"os"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/spf13/cobra"
)

func ShowCmd(opts *app.Options) *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current workspace",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			stokenv, err := env.ReadStokEnv(path)
			if err != nil {
				if os.IsNotExist(err) {
					// no .terraform/environment, so show defaults
					fmt.Fprintln(opts.Out, "default/default")
					return nil
				}
				return err
			}

			fmt.Fprintln(opts.Out, string(stokenv))
			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)

	return cmd
}
