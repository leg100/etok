package cmd

import (
	"fmt"
	"io"

	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
)

func newShowWorkspaceCmd(out io.Writer) *cobra.Command {
	var Path string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current stok workspace",
		Long:  "Show the current stok workspace for this module",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, workspace, err := util.ReadEnvironmentFile(Path)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "%s/%s\n", namespace, workspace)

			return nil
		},
	}
	cmd.Flags().StringVar(&Path, "path", ".", "workspace config path")

	return cmd
}
