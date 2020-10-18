package cmd

import (
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/apps/runner"
	"github.com/spf13/pflag"
)

func init() {
	root.AddChild(NewCmd("runner [command (args)]").
		WithShortHelp("Run the stok runner").
		WithLongHelp("The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.").
		WithHidden().
		WantsKubeClients().
		WithFlags(
			flags.Namespace,
			flags.Path,
			func(fs *pflag.FlagSet, opts *app.Options) {
				fs.StringVar(&opts.Kind, "kind", opts.Kind, "Kubernetes kind to watch")
				fs.StringVar(&opts.Name, "name", opts.Name, "Kubernetes resource name to watch")
				fs.StringVar(&opts.Tarball, "tarball", opts.Tarball, "Tarball filename")
				fs.DurationVar(&opts.TimeoutClient, "timeout", opts.TimeoutClient, "Timeout for client to signal readiness")
			},
		).
		WithApp(runner.NewFromOpts))
}
