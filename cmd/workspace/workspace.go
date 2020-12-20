package workspace

import (
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/spf13/cobra"
)

func WorkspaceCmd(opts *cmdutil.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "etok workspace management",
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
