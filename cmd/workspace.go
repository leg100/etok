package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

func workspaceCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Stok workspace management",
	}
	cmd.AddCommand(newNewWorkspaceCmd(out), newListWorkspaceCmd(out), newDeleteWorkspaceCmd(), newSelectWorkspaceCmd(), newShowWorkspaceCmd(out))

	return cmd
}
