package flags

import (
	"github.com/leg100/stok/pkg/app"
	"github.com/spf13/pflag"
)

type DeferredFlag func(*pflag.FlagSet, *app.Options)

func Path(fs *pflag.FlagSet, opts *app.Options) {
	fs.StringVar(&opts.Path, "path", opts.Path, "Workspace config path")
}

func Namespace(fs *pflag.FlagSet, opts *app.Options) {
	fs.StringVar(&opts.Namespace, "namespace", "default", "Kubernetes namespace")
}

func KubeContext(fs *pflag.FlagSet, opts *app.Options) {
	fs.StringVar(&opts.KubeContext, "context", opts.KubeContext, "Set kube context (defaults to kubeconfig current context)")
}

func Common(fs *pflag.FlagSet, opts *app.Options) {
	fs.BoolVar(&opts.Debug, "debug", opts.Debug, "Enable debug logging")
}
