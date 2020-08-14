package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type FactoryInterface interface {
	NewClient(string) (Client, error)
	GetEventsChannel() chan interface{}
}

type Factory struct {
	events chan interface{}
}

var _ FactoryInterface = &Factory{}

func (f *Factory) NewClient(context string) (Client, error) {
	config, err := config.GetConfigWithContext(context)
	if err != nil {
		return nil, err
	}

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

func (f *Factory) GetEventsChannel() chan interface{} {
	return f.events
}
