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
		ListCmd(opts),
		DeleteCmd(opts),
		ShowCmd(opts),
		SelectCmd(opts),
		DeleteCmd(opts),
	)

	return cmd
}
