package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	environmentFile = ".terraform/environment"
)

func workspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Stok workspace management",
	}
	cmd.AddCommand(newNewWorkspaceCmd(), newListWorkspaceCmd(), newDeleteWorkspaceCmd(), newSelectWorkspaceCmd(), newShowWorkspaceCmd())

	return cmd
}

func validateWorkspaceArg(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("requires a workspace argument")
	}

	return namespacedWorkspace(args[0]).validate()
}
