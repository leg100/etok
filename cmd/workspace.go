package cmd

import (
	"io"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/spf13/cobra"
)

func workspaceCmd(f k8s.FactoryInterface, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Stok workspace management",
	}
	cmd.AddCommand(newNewWorkspaceCmd(f, out), newListWorkspaceCmd(f, out), newDeleteWorkspaceCmd(f), newSelectWorkspaceCmd(), newShowWorkspaceCmd(out))

	return cmd
}
