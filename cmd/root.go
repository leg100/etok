package cmd

import (
	"github.com/leg100/stok/cmd/generate"
	"github.com/leg100/stok/cmd/launcher"
	"github.com/leg100/stok/cmd/manager"
	"github.com/leg100/stok/cmd/runner"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/cmd/workspace"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

func RootCmd(opts *cmdutil.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "stok",
		Version: version.PrintableVersion(),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.Debug {
				log.SetLevel(log.DebugLevel)
				log.Debug("Debug logging enabled")
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.SetUsageFunc((&templater{
		UsageTemplate: MainUsageTemplate(),
	}).UsageFunc())

	cmd.PersistentFlags().BoolVar(&opts.Debug, "debug", false, "Enable debug logging")

	cmd.SetOut(opts.Out)

	cmd.AddCommand(workspace.WorkspaceCmd(opts))
	cmd.AddCommand(generate.GenerateCmd(opts))
	cmd.AddCommand(manager.ManagerCmd(opts))

	runnerCmd, _ := runner.RunnerCmd(opts)
	cmd.AddCommand(runnerCmd)

	launcher.AddCommandsToRoot(cmd, opts)

	return cmd
}
