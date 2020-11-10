package workspace

import (
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/env"
	"github.com/spf13/cobra"
)

func SelectCmd(opts *cmdutil.Options) *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "select <namespace/workspace>",
		Short: "Select a stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := env.Validate(args[0]); err != nil {
				return err
			}
			if err := env.WriteEnvFile(path, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(opts.Out, "Current workspace now: %s\n", args[0])
			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)

	return cmd
}
