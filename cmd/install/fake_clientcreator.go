package install

import (
	"github.com/leg100/etok/pkg/client"
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Implements client.RuntimeClientCreator
type FakeClientCreator struct {
	// Fake objs
	objs []runtimeclient.Object
}

func NewFakeClientCreator(objs ...runtimeclient.Object) client.RuntimeClientCreator {
	return &FakeClientCreator{objs: objs}
}

func (f *FakeClientCreator) CreateRuntimeClient(kubeCtx string) (*client.Client, error) {
	return &client.Client{
		Config:        &rest.Config{},
		RuntimeClient: fake.NewClientBuilder().WithObjects(f.objs...).Build(),
	}, nil
}
