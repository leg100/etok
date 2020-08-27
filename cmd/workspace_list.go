package cmd

import (
	"flag"
	"io"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/workspace"
	"github.com/spf13/cobra"
)

func newListWorkspaceCmd(f k8s.FactoryInterface, out io.Writer) *cobra.Command {
	listWorkspace := &workspace.ListWorkspace{Factory: f, Out: out}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			return listWorkspace.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&listWorkspace.Path, "path", ".", "workspace config path")
	cmd.Flags().StringVar(&listWorkspace.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

	// Add flags registered by imported packages (controller-runtime)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	return cmd
}
