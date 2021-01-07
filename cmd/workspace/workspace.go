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

func WorkspaceCmd(opts *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Etok workspace management",
	}

	nc, _ := newCmd(opts)
	cmd.AddCommand(nc)

	cmd.AddCommand(
		listCmd(opts),
		deleteCmd(opts),
		showCmd(opts),
		selectCmd(opts),
	)

	return cmd
}
