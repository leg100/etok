package fake

import (
	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type Factory struct {
	objs    []runtime.Object
	Client  runtimeclient.Client
	manager *k8s.Manager
	Context string
}

var _ k8s.FactoryInterface = &Factory{}

func NewFactory(objs ...runtime.Object) *Factory {
	return &Factory{
		objs: objs,
	}
}

func (f *Factory) NewConfig(context string) (*rest.Config, error) {
	f.Context = context
	return &rest.Config{}, nil
}

func (f *Factory) NewClient(config *rest.Config) (k8s.Client, error) {
	f.Client = runtimefake.NewFakeClientWithScheme(scheme.Scheme, f.objs...)

	return &client{factory: f, Client: f.Client, config: config}, nil
}

func (f *Factory) NewManager(config *rest.Config, namespace string) (*k8s.Manager, error) {
	f.manager = &k8s.Manager{
		Cache: NewCache(scheme.Scheme),
		Objs:  f.objs,
	}

	return f.manager, nil
}

// A (naive) implementation of the algorithm that k8s uses to generate a unique name on the
// server side when `generateName` is specified. Allows us to generate a unique name client-side
// for our k8s resources.
func (f *Factory) GenerateName(kind string) string {
	return GenerateName(kind)
}

func GenerateName(kind string) string {
	return run.GenerateName(kind, "12345")
}
