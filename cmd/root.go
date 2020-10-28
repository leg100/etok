package cmd

import (
	"github.com/leg100/stok/cmd/generate"
	"github.com/leg100/stok/cmd/launcher"
	"github.com/leg100/stok/cmd/manager"
	"github.com/leg100/stok/cmd/runner"
	"github.com/leg100/stok/cmd/workspace"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/log"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

func RootCmd(opts *app.Options) *cobra.Command {
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
		SilenceUsage: true,
	}

	cmd.PersistentFlags().BoolVar(&opts.Debug, "debug", false, "Enable debug logging")

	cmd.SetOut(opts.Out)

	cmd.AddCommand(workspace.WorkspaceCmd(opts))
	cmd.AddCommand(generate.GenerateCmd(opts))
	cmd.AddCommand(launcher.LauncherCmds(opts)...)
	cmd.AddCommand(manager.ManagerCmd(opts))

	runnerCmd, _ := runner.RunnerCmd(opts)
	cmd.AddCommand(runnerCmd)

	return cmd
}
