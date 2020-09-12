package k8s

import (
	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type FactoryInterface interface {
	NewConfig(string) (*rest.Config, error)
	NewClient(*rest.Config) (Client, error)
	NewManager(*rest.Config, string) (*Manager, error)
	GenerateName(string) string
}

type Factory struct {
	objs    []runtime.Object
	manager *Manager
}

var _ FactoryInterface = &Factory{}

func (f *Factory) NewConfig(context string) (*rest.Config, error) {
	return config.GetConfigWithContext(context)
}

func (f *Factory) NewClient(config *rest.Config) (Client, error) {
	// TODO: scheme should be fed through from tested code
	rc, err := runtimeclient.New(config, runtimeclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	kc, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{Client: rc, kc: kc, config: config}, nil
}

func (f *Factory) NewManager(config *rest.Config, namespace string) (*Manager, error) {
	mapper, err := apiutil.NewDynamicRESTMapper(config)
	if err != nil {
		return nil, err
	}

	cache, err := runtimecache.New(config, runtimecache.Options{
		Scheme:    scheme.Scheme,
		Mapper:    mapper,
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	f.manager = &Manager{Cache: cache, Objs: f.objs}
	return f.manager, nil
}

// A (naive) implementation of the algorithm that k8s uses to generate a unique name on the
// server side when `generateName` is specified. Allows us to generate a unique name client-side
// for our k8s resources.
func (f *Factory) GenerateName(kind string) string {
	return run.GenerateName(kind, util.GenerateRandomString(5))
}
