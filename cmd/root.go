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
	"github.com/leg100/stok/cmd/generate"
	"github.com/leg100/stok/cmd/launcher"
	"github.com/leg100/stok/cmd/manager"
	"github.com/leg100/stok/cmd/runner"
	"github.com/leg100/stok/cmd/workspace"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/version"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func extractExitCodeFromError(err error) int {
	var exiterr *exec.ExitError
	if errors.As(err, &exiterr) {
		return exiterr.ExitCode()
	}
	return 1
}

func Execute(ctx context.Context, args []string) int {
	if err := run(ctx, args); err != nil {
		log.WithError(err).Error("Fatal error")

		return extractExitCodeFromError(err)
	}
	return 0
}

func run(ctx context.Context, args[]string) int {
	rootcmd := &ffcli.Command{
		Name:       "stok",
		ShortUsage: "stok <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		LongHelp: "Supercharge terraform on kubernetes",
	}

	rootcmd.Subcommands = []*ffcli.Command{
		launcher.NewLauncherCmds(cc.cmd, args),
		workspace.WorkspaceCmd(out),
		generate.GenerateCmd(out),
		runner.NewRunnerCmd(),
		manager.NewOperatorCmd(),
	}

	if err := rootCommand.Parse(args); err != nil {
		return err
	}

	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	kc, err := k8s.KubeClient()
	if err != nil {
		return err
	}

}

func newStokCmd(args []string, out, errout io.Writer) *stokCmd {
	log.SetHandler(cli.New(out, errout))

	var (
		rootFlagSet = flag.NewFlagSet
		versionFlag = rootFlagSet

	if cc.debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug logging enabled")
	}
	return nil
	cc.cmd.PersistentFlags().BoolVar(&cc.debug, "debug", false, "Enable debug logging")


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
