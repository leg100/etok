package util

import (
	"fmt"
	"io"

	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/k8s/etokclient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// TODO: move constants somewhere more appropriate
const (
	// HandshakeString is the string that the runner expects to receive via stdin prior to running.
	HandshakeString = "opensesame"
)

// Factory pertaining to etok apps
type Factory struct {
	// Deferred creation of clients (k8s and etok clientsets)
	client.ClientCreator

	// Deferred creation of controller-runtime clients
	client.RuntimeClientCreator

	*ClientBuilder

	IOStreams

	Verbosity int
}

// IOStreams provides the standard names for iostreams.  This is useful for embedding and for unit testing.
// Inconsistent and different names make it hard to read and review code
type IOStreams struct {
	// In think, os.Stdin
	In io.Reader
	// Out think, os.Stdout
	Out io.Writer
	// ErrOut think, os.Stderr
	ErrOut io.Writer
}

func NewFactory(out, errout io.Writer, in io.Reader) *Factory {
	f := &Factory{
		ClientCreator:        client.NewClientCreator(),
		RuntimeClientCreator: client.NewRuntimeClientCreator(),
		IOStreams: IOStreams{
			Out:    out,
			ErrOut: errout,
			In:     in,
		},
	}
	// Set logger output device
	klog.SetOutput(f.Out)
	return f
}

func NewFakeFactory(out io.Writer, objs ...runtime.Object) *Factory {
	return &Factory{
		ClientCreator: client.NewFakeClientCreator(objs...),
		IOStreams: IOStreams{
			Out: out,
		},
	}
}

// ClientBuilderInterface is capable of constructing both the built-in client
// and the generated etok client
type ClientBuilderInterface interface {
	NewKubeClient(string) (kubernetes.Interface, error)
	NewEtokClient(string) (etokclient.Interface, error)
}

// ClientBuilder makes real clients
type ClientBuilder struct {
	cfg *rest.Config
}

func (cb *ClientBuilder) NewKubeClient(kubeCtx string) (kubernetes.Interface, error) {
	cfg, err := cb.getConfig(kubeCtx)
	if err != nil {
		return nil, err
	}

	kubeclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating built-in kubernetes client: %w", err)
	}

	return kubeclient, nil
}

func (cb *ClientBuilder) NewEtokClient(kubeCtx string) (etokclient.Interface, error) {
	cfg, err := cb.getConfig(kubeCtx)
	if err != nil {
		return nil, err
	}

	etokclient, err := etokclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating etok kubernetes client: %w", err)
	}

	return etokclient, nil
}

// getConfig creates a k8s config from a kube context. It'll cache the config
// the first time it is created; further calls retrieve the cached config.
func (cb *ClientBuilder) getConfig(kubeCtx string) (*rest.Config, error) {
	cfg, err := config.GetConfigWithContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes client config: %w", err)
	}

	// Cache it
	cb.cfg = cfg

	return cfg, nil
}
