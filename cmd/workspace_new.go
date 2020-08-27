package cmd

import (
	"flag"
	"io"
	"time"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/workspace"
	"github.com/spf13/cobra"
)

func newNewWorkspaceCmd(f k8s.FactoryInterface, out io.Writer) *cobra.Command {
	newWorkspace := &workspace.NewWorkspace{Factory: f, Out: out}

	cmd := &cobra.Command{
		Use:   "new <workspace>",
		Short: "Create a new stok workspace",
		Long:  "Deploys a Workspace resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			debug, err := cmd.InheritedFlags().GetBool("debug")
			if err != nil {
				return err
			}
			newWorkspace.Debug = debug

			newWorkspace.Name = args[0]

			return newWorkspace.Run(cmd.Context())
		},
	}
	cmd.Flags().StringVar(&newWorkspace.Path, "path", ".", "workspace config path")
	cmd.Flags().StringVar(&newWorkspace.Namespace, "namespace", "default", "Kubernetes namespace of workspace")

	cmd.Flags().StringVar(&newWorkspace.WorkspaceSpec.ServiceAccountName, "service-account", "", "Name of ServiceAccount")
	cmd.Flags().StringVar(&newWorkspace.WorkspaceSpec.SecretName, "secret", "", "Name of Secret containing credentials")

	cmd.Flags().StringVar(&newWorkspace.WorkspaceSpec.Cache.Size, "size", "1Gi", "Size of PersistentVolume for cache")
	cmd.Flags().StringVar(&newWorkspace.WorkspaceSpec.Cache.StorageClass, "storage-class", "", "StorageClass of PersistentVolume for cache")
	cmd.Flags().DurationVar(&newWorkspace.Timeout, "timeout", 10*time.Second, "Time to wait for workspace to be healthy")

	// Validate
	cmd.Flags().StringVar(&newWorkspace.WorkspaceSpec.TimeoutClient, "timeout-client", "10s", "timeout for client to signal readiness")

	cmd.Flags().DurationVar(&newWorkspace.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
	cmd.Flags().StringVar(&newWorkspace.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

	cmd.Flags().StringVar(&newWorkspace.WorkspaceSpec.Backend.Type, "backend-type", "local", "Set backend type")
	cmd.Flags().StringToStringVar(&newWorkspace.WorkspaceSpec.Backend.Config, "backend-config", map[string]string{}, "Set backend config (command separated key values, e.g. bucket=gcs,prefix=dev")

	// Add flags registered by imported packages (controller-runtime)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	return cmd
}
