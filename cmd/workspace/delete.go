package workspace

import (
	"flag"

	"github.com/leg100/stok/pkg/workspace"
	"github.com/spf13/cobra"
)

func newDeleteWorkspaceCmd() *cobra.Command {
	deleteWorkspace := &workspace.DeleteWorkspace{}

	cmd := &cobra.Command{
		Use:   "delete <namespace/workspace>",
		Short: "Delete a stok workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			deleteWorkspace.Name = args[0]

			return deleteWorkspace.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&deleteWorkspace.Path, "path", ".", "workspace config path")
	cmd.Flags().StringVar(&deleteWorkspace.Namespace, "namespace", "default", "Kubernetes namespace of workspace")
	cmd.Flags().StringVar(&deleteWorkspace.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

	// Add flags registered by imported packages (controller-runtime)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	return cmd
}
