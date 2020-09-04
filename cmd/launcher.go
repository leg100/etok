package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/launcher"
	"github.com/leg100/stok/util"
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
				// If either namespace or workspace has not been set by user, then try to load them
				// from an environment file
				namespace, workspace, err := util.ReadEnvironmentFile(launcher.Path)
				if err != nil && !os.IsNotExist(err) {
					// It's ok for an environment file to not exist, but not any other error
					return err
				}
				if !cmd.Flags().Changed("namespace") {
					launcher.Namespace = namespace
				}
				if !cmd.Flags().Changed("workspace") {
					launcher.Workspace = workspace
				}

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
		cmd.Flags().StringVar(&launcher.Namespace, "namespace", "default", "Kubernetes namespace of workspace")

		cmd.Flags().StringVar(&launcher.Workspace, "workspace", "default", "Workspace name")
		cmd.Flags().StringVar(&launcher.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

		// Add flags registered by imported packages (controller-runtime)
		cmd.Flags().AddGoFlagSet(flag.CommandLine)

		cmds = append(cmds, cmd)
	}

	return cmds
}
