package cmd

import (
	"flag"
	"strconv"

	"github.com/leg100/etok/cmd/github"
	"github.com/leg100/etok/cmd/install"
	"github.com/leg100/etok/cmd/manager"
	"github.com/leg100/etok/cmd/runner"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/cmd/workspace"
	"github.com/leg100/etok/pkg/executor"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

func RootCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "etok",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			f.Verbosity, _ = strconv.Atoi(cmd.Flags().Lookup("v").Value.String())
		},
	}

	// Pull in klog's flags
	klogfs := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(klogfs)
	cmd.PersistentFlags().AddGoFlagSet(klogfs)

	cmd.SetOut(f.Out)

	cmd.AddCommand(versionCmd(f))

	cmd.AddCommand(workspace.WorkspaceCmd(f))
	cmd.AddCommand(manager.ManagerCmd(f))

	runnerCmd, _ := runner.RunnerCmd(f)
	cmd.AddCommand(runnerCmd)

	installCmd, _ := install.InstallCmd(f)
	cmd.AddCommand(installCmd)

	cmd.AddCommand(github.GithubCmd(f))

	// Terraform commands (and shell command)
	launcher.AddToRoot(cmd, f)
	// terraform fmt
	cmd.AddCommand(launcher.FmtCmd(&executor.Exec{IOStreams: f.IOStreams}))

	return cmd
}
