package cmd

import (
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
)

func newSelectWorkspaceCmd() *cobra.Command {
	var Path, Namespace string

	cmd := &cobra.Command{
		Use:   "select <namespace/workspace>",
		Short: "Select a stok workspace",
		Long:  "Sets the current stok workspace for this module",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return util.WriteEnvironmentFile(Path, Namespace, args[0])
		},
	}
	cmd.Flags().StringVar(&Path, "path", ".", "workspace config path")
	cmd.Flags().StringVar(&Namespace, "namespace", "default", "Kubernetes namespace of workspace")

	return cmd
}
