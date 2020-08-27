package cmd

import (
	"flag"
	"fmt"
	"time"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/launcher"
	"github.com/spf13/cobra"
)

func newLauncherCmds(f k8s.FactoryInterface) []*cobra.Command {
	var cmds []*cobra.Command

	for _, kind := range command.CommandKinds {
		launcher := &launcher.Launcher{Kind: kind, Factory: f}

		cmd := &cobra.Command{
			Use:   command.CommandKindToCLI(kind),
			Short: fmt.Sprintf("Run %s", command.CommandKindToCLI(kind)),
			RunE: func(cmd *cobra.Command, args []string) error {
				debug, err := cmd.InheritedFlags().GetBool("debug")
				if err != nil {
					return err
				}
				launcher.Debug = debug
				launcher.Args = args
				return launcher.Run(cmd.Context())
			},
		}
		cmd.Flags().StringVar(&launcher.Path, "path", ".", "terraform config path")
		cmd.Flags().DurationVar(&launcher.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
		cmd.Flags().DurationVar(&launcher.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
		cmd.Flags().DurationVar(&launcher.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
		// TODO: rename to timeout-pending (enqueue is too similar sounding to queue)
		cmd.Flags().DurationVar(&launcher.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")
		cmd.Flags().StringVar(&launcher.Namespace, "namespace", "", "Kubernetes namespace of workspace (defaults to namespace set in .terraform/environment, or \"default\")")

		cmd.Flags().StringVar(&launcher.Workspace, "workspace", "", "Workspace name (defaults to workspace set in .terraform/environment or, \"default\")")
		cmd.Flags().StringVar(&launcher.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

		// Add flags registered by imported packages (controller-runtime)
		cmd.Flags().AddGoFlagSet(flag.CommandLine)

		cmds = append(cmds, cmd)
	}

	return cmds
}
