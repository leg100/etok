package workspace

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

const (
	// default namespace workspaces are created in or if .terraform/environment
	// is not found
	defaultNamespace = "default"

	// default workspace if .terraform/environment is not found
	defaultWorkspace = "default"
)

func WorkspaceCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		Aliases: []string{"ws"},
		Short:   "Etok workspace management",
	}

	nc, _ := newCmd(f)
	cmd.AddCommand(nc)

	cmd.AddCommand(
		listCmd(f),
		deleteCmd(f),
		showCmd(f),
		selectCmd(f),
	)

	return cmd
}
