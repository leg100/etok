package workspace

import (
	"fmt"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/spf13/cobra"
)

func selectCmd(f *cmdutil.Factory) *cobra.Command {
	var path string
	var namespace = defaultNamespace

	cmd := &cobra.Command{
		Use:   "select <workspace>",
		Short: "Select an etok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validates parameters
			etokenv, err := env.New(namespace, args[0])
			if err != nil {
				return err
			}

			if err := etokenv.Write(path); err != nil {
				return err
			}
			fmt.Fprintf(f.Out, "Current workspace now: %s\n", etokenv)
			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)
	flags.AddNamespaceFlag(cmd, &namespace)

	return cmd
}
