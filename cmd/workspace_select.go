package cmd

import (
	"github.com/spf13/cobra"
)

type selectWorkspaceCmd struct {
	Path string

	cmd *cobra.Command
}

func newSelectWorkspaceCmd() *cobra.Command {
	cc := &selectWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "select <namespace/workspace>",
		Short: "Select a stok workspace",
		Long:  "Sets the current stok workspace for this module",
		Args: func(cmd *cobra.Command, args []string) error {
			return validateWorkspaceArg(args)
		},
		RunE: cc.doSelectWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")

	return cc.cmd
}

func (t *selectWorkspaceCmd) doSelectWorkspace(cmd *cobra.Command, args []string) error {
	return namespacedWorkspace(args[0]).writeEnvironmentFile(t.Path)
}
