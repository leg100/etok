package cmd

import (
	"context"
	"flag"

	"github.com/leg100/stok/pkg/generate"
	"github.com/leg100/stok/pkg/options"
	"github.com/leg100/stok/version"
)

func init() {
	generateCmd.AddChild(
		NewCmd("operator").
			WithShortUsage("new <[namespace/]workspace>").
			WithShortHelp("Generate operator's kubernetes resources").
			WithFlags(func(fs *flag.FlagSet, opts *options.StokOptions) {
				fs.StringVar(&opts.Name, "name", "stok-operator", "Name for kubernetes resources")
				fs.StringVar(&opts.Namespace, "namespace", "default", "Kubernetes namespace for resources")
				fs.StringVar(&opts.Image, "image", version.Image, "Docker image used for both the operator and the runner")
			}).
			WithOneArg().
			WithExec(func(ctx context.Context, opts *options.StokOptions) error {
				return (&generate.Operator{
					Name:      opts.Name,
					Namespace: opts.Namespace,
					Image:     opts.Image,
					Out:       opts.Out,
				}).Generate()
			}))
}
