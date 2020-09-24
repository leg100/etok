package workspace

import (
	"fmt"
	"io"

	"github.com/leg100/stok/pkg/env"
	"github.com/spf13/cobra"
)

func newShowWorkspaceCmd(out io.Writer) *cobra.Command {
	var Path string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current stok workspace",
		Long:  "Show the current stok workspace for this module",
		RunE: func(cmd *cobra.Command, args []string) error {
			stokenv, err := env.ReadStokEnv(Path)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, string(stokenv))

			return nil
		},
	}
	cmd.Flags().StringVar(&Path, "path", ".", "workspace config path")

	return cmd
}
