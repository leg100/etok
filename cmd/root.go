package cmd

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"

	"github.com/apex/log"
	"github.com/leg100/stok/cmd/generate"
	"github.com/leg100/stok/cmd/launcher"
	"github.com/leg100/stok/cmd/manager"
	"github.com/leg100/stok/cmd/options"
	"github.com/leg100/stok/cmd/runner"
	"github.com/leg100/stok/cmd/workspace"
	"github.com/leg100/stok/logging/handlers/cli"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/util/slice"
	"github.com/peterbourgon/ff"
	"github.com/peterbourgon/ff/v3/ffcli"
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

func run(ctx context.Context, args []string) error {
	log.SetHandler(cli.New(os.Stdout, os.Stderr))

	fs := flag.NewFlagSet("stok", flag.ExitOnError)

	var globalOpts options.GlobalOpts
	var kubeOpts options.KubeOpts

	globalOpts.AddFlags(fs)
	rootcmd := &ffcli.Command{
		Name:       "stok",
		ShortUsage: "stok <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		LongHelp:   "Supercharge terraform on kubernetes",
		Options:    []ff.Option{ff.WithEnvVarPrefix("STOK")},
	}

	rootcmd.Subcommands = []*ffcli.Command{
		launcher.NewLauncherCmds(globalOpts, kubeOpts),
		workspace.WorkspaceCmd(globalOpts, kubeOpts),
		generate.GenerateCmd(globalOpts),
		runner.NewRunnerCmd(globalOpts, kubeOpts),
		manager.NewOperatorCmd(globalOpts),
	}

	if err := parse(rootcmd, args); err != nil {
		return err
	}

	if globalOpts.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug logging enabled")
	}

	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	kc, err := k8s.KubeClient()
	if err != nil {
		return err
	}

	kubeOpts.StokClient = sc
	kubeOpts.KubeClient = kc

	return rootcmd.Run(ctx)
}

func parse(rootcmd *ffcli.Command, args []string) error {
	if isLauncherCmd(args) {
		// Swap terraform args to instead follow a terminator (--), and stok args
		// to precede a terminator
		args = append([]string{args[0]}, swapArgs(args[1:])...)
	}

	return rootcmd.Parse(args)
}

// Parse args to determine if it'll invoke a launcher command, i.e. terraform commands
func isLauncherCmd(args []string) bool {
	return len(args) > 0 && slice.ContainsString(run.TerraformCommands, args[0])
}

// Swap around args:
// (a) If terminator (--) is found, then swap the args either side of terminator
// (b) If terminator is not found, then add one before any args
func swapArgs(args []string) []string {
	if i := slice.StringIndex(args, "--"); i > -1 {
		return append(args[i+1:], args[:i]...)
	} else {
		return append([]string{"--"}, args...)
	}
}
