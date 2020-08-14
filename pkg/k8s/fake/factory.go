package fake

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type Factory struct {
	Objs           []runtime.Object
	Client         runtimeclient.Client
	Context        string
	getReactors    Reactors
	createReactors Reactors
	deleteReactors Reactors
	events         chan interface{}
}

var _ k8s.FactoryInterface = &Factory{}

func NewFactory(objs ...runtime.Object) *Factory {
	return &Factory{Objs: objs}
}

func (f *Factory) GetEventsChannel() chan interface{} {
	return f.events
}

func (f *Factory) NewClient(context string) (k8s.Client, error) {
	rc := runtimefake.NewFakeClientWithScheme(scheme.Scheme, f.Objs...)
	f.Client = rc
	f.Context = context

	return &client{factory: f, Client: rc, config: &rest.Config{}}, nil
}

func (f *Factory) AddReactor(action string, reactor Reactor) *Factory {
	switch action {
	case "create":
		f.createReactors = append(f.createReactors, reactor)
	case "delete":
		f.deleteReactors = append(f.deleteReactors, reactor)
	case "get":
		f.getReactors = append(f.getReactors, reactor)
	default:
		panic(fmt.Sprintf("no reactor support for %s", action))
	}
	return f
}

func (f *Factory) InjectObj(obj metav1.Object) {
	f.events <- obj
}
