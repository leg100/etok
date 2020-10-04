package cmd

import (
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/apps/runner"
)

func init() {
	root.AddChild(NewCmd("runner").
		WithShortUsage("runner [command (args)]").
		WithShortHelp("Run the stok runner").
		WithLongHelp("The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.").
		WithFlags(
			flags.KubeContext,
		).
		WithEnvVars().
		WithApp(runner.NewFromOptions))
}
