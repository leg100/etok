package cmd

import (
	"context"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/generate"
	"github.com/spf13/pflag"
)

func init() {
	generateCmd.AddChild(
		NewCmd("operator").
			WithShortHelp("Generate operator's kubernetes resources").
			WithFlags(func(fs *pflag.FlagSet, opts *app.Options) {
				fs.StringVar(&opts.Name, "name", "stok-operator", "Name for kubernetes resources")
				fs.StringVar(&opts.Namespace, "namespace", opts.Namespace, "Kubernetes namespace for resources")
				fs.StringVar(&opts.Image, "image", opts.Image, "Docker image used for both the operator and the runner")
			}).
			WithExec(func(ctx context.Context, opts *app.Options) error {
				return (&generate.Operator{
					Name:      opts.Name,
					Namespace: opts.Namespace,
					Image:     opts.Image,
					Out:       opts.Out,
				}).Generate()
			}))
}
