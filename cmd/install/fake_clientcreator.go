package install

import (
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Implements client.RuntimeClientCreator
type FakeClientCreator struct {
	// Fake objs
	objs []runtime.Object
}

func NewFakeClientCreator(objs ...runtime.Object) client.RuntimeClientCreator {
	return &FakeClientCreator{objs: objs}
}

func (f *FakeClientCreator) CreateRuntimeClient(kubeCtx string) (*client.Client, error) {
	return &client.Client{
		Config:        &rest.Config{},
		RuntimeClient: fake.NewFakeClientWithScheme(scheme.Scheme, f.objs...),
	}, nil
}
