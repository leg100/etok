package flags

import (
	"flag"

	"github.com/leg100/stok/pkg/options"
)

type DeferredFlag func(*flag.FlagSet, *options.StokOptions)

func Path(fs *flag.FlagSet, opts *options.StokOptions) {
	fs.StringVar(&opts.Path, "path", ".", "Workspace config path")
}

func Namespace(fs *flag.FlagSet, opts *options.StokOptions) {
	fs.StringVar(&opts.Namespace, "namespace", "default", "Kubernetes namespace")
}

func KubeContext(fs *flag.FlagSet, opts *options.StokOptions) {
	fs.StringVar(&opts.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")
}

func Common(fs *flag.FlagSet, opts *options.StokOptions) {
	fs.BoolVar(&opts.Debug, "debug", false, "Enable debug logging")
	fs.BoolVar(&opts.Help, "h", false, "print usage")
}

func Version(fs *flag.FlagSet, opts *options.StokOptions) {
	fs.BoolVar(&opts.Version, "v", false, "print version")
}
