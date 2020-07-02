package cmd

import (
	"github.com/spf13/cobra"
)

type selectWorkspaceCmd struct {
	Path      string
	Namespace string

	cmd *cobra.Command
}

func newSelectWorkspaceCmd() *cobra.Command {
	cc := &selectWorkspaceCmd{}
	cc.cmd = &cobra.Command{
		Use:   "select <namespace/workspace>",
		Short: "Select a stok workspace",
		Long:  "Sets the current stok workspace for this module",
		Args:  cobra.ExactArgs(1),
		RunE:  cc.doSelectWorkspace,
	}
	cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "workspace config path")
	cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "default", "Kubernetes namespace of workspace")

	return cc.cmd
}

func (t *selectWorkspaceCmd) doSelectWorkspace(cmd *cobra.Command, args []string) error {
	return writeEnvironmentFile(t.Path, t.Namespace, args[0])
}
