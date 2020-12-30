package cmd

import (
	"flag"
	"strconv"

	"github.com/leg100/etok/cmd/generate"
	"github.com/leg100/etok/cmd/launcher"
	"github.com/leg100/etok/cmd/manager"
	"github.com/leg100/etok/cmd/runner"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/cmd/workspace"
	"github.com/leg100/etok/pkg/executor"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func RootCmd(opts *cmdutil.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "etok",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			opts.Verbosity, _ = strconv.Atoi(cmd.Flags().Lookup("v").Value.String())
		},
	}

	// Pull in klog's flags
	klogfs := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(klogfs)
	cmd.PersistentFlags().AddGoFlagSet(klogfs)

	cmd.SetOut(opts.Out)

	cmd.AddCommand(versionCmd(opts))
	cmd.AddCommand(workspace.WorkspaceCmd(opts))
	cmd.AddCommand(generate.GenerateCmd(opts))
	cmd.AddCommand(manager.ManagerCmd(opts))

	runnerCmd, _ := runner.RunnerCmd(opts)
	cmd.AddCommand(runnerCmd)

	// Terraform commands (and shell command)
	launcher.AddToRoot(cmd, opts)
	// terraform fmt
	cmd.AddCommand(launcher.FmtCmd(&executor.Exec{IOStreams: opts.IOStreams}))

	return cmd
}
