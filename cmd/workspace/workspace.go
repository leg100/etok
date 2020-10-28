package workspace

import (
	"github.com/leg100/stok/pkg/app"
	"github.com/spf13/cobra"
)

func WorkspaceCmd(opts *app.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Stok workspace management",
	}

	newCmd, _ := NewCmd(opts)
	cmd.AddCommand(
		newCmd,
	)
	return cmd
}
