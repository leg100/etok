package cmd

import (
	"fmt"
	"os/exec"

	"github.com/apex/log"
	"github.com/leg100/stok/cmd/manager"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

type stokCmd struct {
	debug bool
	cmd   *cobra.Command
	exit  func(int)
}

func Execute(args []string, exit func(int)) {
	log.SetHandler(cli.Default)

	newStokCmd(exit).Execute(args)
}

func (cc *stokCmd) Execute(args []string) {
	cc.cmd.SetArgs(args)

	if err := cc.cmd.Execute(); err != nil {
		var code = 1
		if exiterr, ok := err.(*exec.ExitError); ok {
			code = exiterr.ExitCode()
		}
		log.WithError(err).Error("")
		cc.exit(code)
	}
}

func newStokCmd(exit func(int)) *stokCmd {
	cc := &stokCmd{exit: exit}

	cc.cmd = &cobra.Command{
		Use:   "stok",
		Short: "Supercharge terraform on kubernetes",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cc.debug {
				log.SetLevel(log.DebugLevel)
				log.Debug("debug logging enabled")
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       fmt.Sprintf("%s\t%s", version.Version, version.Commit),
	}

	cc.cmd.PersistentFlags().BoolVar(&cc.debug, "debug", false, "Enable debug logging")

	childCommands := append(
		newTerraformCmds(),
		workspaceCmd(),
		generateCmd(),
		newRunnerCmd(),
		manager.NewOperatorCmd())

	cc.cmd.AddCommand(childCommands...)

	return cc
}
