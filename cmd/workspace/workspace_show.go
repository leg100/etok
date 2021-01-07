package workspace

import (
	"fmt"
	"os"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/spf13/cobra"
)

func showCmd(opts *cmdutil.Factory) *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current workspace",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			etokenv, err := env.Read(path)
			if err != nil {
				if os.IsNotExist(err) {
					// no .terraform/environment, so show defaults
					fmt.Fprintf(opts.Out, "%s/%s\n", defaultNamespace, defaultWorkspace)
					return nil
				}
				return fmt.Errorf("failed reading contents of %s: %w", path, err)
			}

			fmt.Fprintln(opts.Out, etokenv)
			return nil
		},
	}

	flags.AddPathFlag(cmd, &path)

	return cmd
}
