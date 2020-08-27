package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/apex/log"
	"github.com/leg100/stok/cmd/manager"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type stokCmd struct {
	debug bool
	cmd   *cobra.Command
}

func Execute(args []string) int {
	code, _ := newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr).Execute(args)
	return code
}

// Run stok command with args, unwrap exit code from error, and return both code and error
func (cc *stokCmd) Execute(args []string) (int, error) {
	cc.cmd.SetArgs(args)

	// Create context, and cancel if interrupt is received
	ctx, cancel := context.WithCancel(context.Background())
	catchCtrlC(cancel)

	if err := cc.cmd.ExecuteContext(ctx); err != nil {
		log.WithError(err).Error("Fatal error")

		var exiterr *exec.ExitError
		if errors.As(err, &exiterr) {
			return exiterr.ExitCode(), err
		}
		return 1, err
	}
	return 0, nil
}

func newStokCmd(f k8s.FactoryInterface, out, errout io.Writer) *stokCmd {
	log.SetHandler(cli.New(out, errout))

	cc := &stokCmd{}

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
		newLauncherCmds(f),
		workspaceCmd(f, out),
		generateCmd(out),
		newRunnerCmd(f),
		manager.NewOperatorCmd())

	cc.cmd.AddCommand(childCommands...)

	setFlagsFromEnvVariables(cc.cmd)

	return cc
}

// Each flag can also be set with an env variable whose name starts with `STOK_`.
func setFlagsFromEnvVariables(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		envVar := FlagToEnvVarName(f)
		if val, present := os.LookupEnv(envVar); present {
			rootCmd.PersistentFlags().Set(f.Name, val)
		}
	})
	for _, cmd := range rootCmd.Commands() {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			envVar := FlagToEnvVarName(f)
			if val, present := os.LookupEnv(envVar); present {
				cmd.Flags().Set(f.Name, val)
			}
		})
	}
}

func FlagToEnvVarName(f *pflag.Flag) string {
	return fmt.Sprintf("STOK_%s", strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
}
