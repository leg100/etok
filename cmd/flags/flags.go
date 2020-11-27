package flags

import (
	"github.com/spf13/cobra"
)

func AddPathFlag(cmd *cobra.Command, path *string) {
	cmd.Flags().StringVar(path, "path", ".", "Workspace config path")
}

func AddNamespaceFlag(cmd *cobra.Command, namespace *string) {
	cmd.Flags().StringVar(namespace, "namespace", "default", "Kubernetes namespace")
}

func AddKubeContextFlag(cmd *cobra.Command, kubeContext *string) {
	cmd.Flags().StringVar(kubeContext, "context", "", "Set kube context (defaults to kubeconfig current context)")
}

func AddDisableResourceCleanupFlag(cmd *cobra.Command, disable *bool) {
	cmd.Flags().BoolVar(disable, "no-cleanup", false, "Do not delete kubernetes resources upon error")
}
