package options

import (
	"flag"

	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
)

type GlobalOpts struct {
	Debug bool
}

func (opts *GlobalOpts) AddFlags(fs *flag.FlagSet) {
	fs.BoolVar(&opts.Debug, "debug", false, "Enable debug logging")
}

type KubeOpts struct {
	Context    string
	Namespace  string
	StokClient stokclient.Interface
	KubeClient kubernetes.Interface
}

func (opts *KubeOpts) AddFlags(fs *flag.FlagSet) {
	fs.StringVar(&opts.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")
	fs.StringVar(&opts.Namespace, "namespace", "default", "Kubernetes namespace")
}
