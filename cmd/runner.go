package cmd

import (
	"fmt"
	"time"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/runner"
	"github.com/spf13/cobra"
)

func newRunnerCmd(f k8s.FactoryInterface) *cobra.Command {
	runner := &runner.Runner{Factory: f}

	cmd := &cobra.Command{
		// TODO: what is the syntax for stating at least one command must be provided?
		Use:           "runner [command (args)]",
		Short:         "Run the stok runner",
		Long:          "The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runner.Args = args
			if err := runner.Run(cmd.Context()); err != nil {
				return fmt.Errorf("runner: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&runner.Path, "path", ".", "Workspace config path")
	cmd.Flags().StringVar(&runner.Tarball, "tarball", "", "Extract specified tarball file to workspace path")

	cmd.Flags().BoolVar(&runner.NoWait, "no-wait", false, "Disable polling resource for client annotation")
	cmd.Flags().StringVar(&runner.Name, "name", "", "Name of command resource")
	cmd.Flags().StringVar(&runner.Namespace, "namespace", "default", "Namespace of command resource")
	cmd.Flags().StringVar(&runner.Kind, "kind", "", "Kind of command resource")
	cmd.Flags().StringVar(&runner.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")
	cmd.Flags().DurationVar(&runner.Timeout, "timeout", 10*time.Second, "Timeout on waiting for client to confirm readiness")
	return cmd
}
