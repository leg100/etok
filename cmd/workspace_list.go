package cmd

import (
	"flag"
	"io"

	"github.com/leg100/stok/pkg/k8s/config"
	"github.com/leg100/stok/pkg/workspace"
	"github.com/spf13/cobra"
)

func newListWorkspaceCmd(out io.Writer) *cobra.Command {
	listWorkspace := &workspace.ListWorkspace{Out: out}

	var kubeContext string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			config.SetContext(kubeContext)

			return listWorkspace.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&listWorkspace.Path, "path", ".", "workspace config path")
	cmd.Flags().StringVar(&kubeContext, "context", "", "Set kube context (defaults to kubeconfig current context)")

	// Add flags registered by imported packages (controller-runtime)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	return cmd
}
