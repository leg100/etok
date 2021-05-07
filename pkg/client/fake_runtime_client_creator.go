package client

import (
	"github.com/leg100/etok/pkg/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Implements RuntimeClientCreator
type fakeRuntimeClientCreator struct {
	objs []runtimeclient.Object
}

func NewFakeRuntimeClientCreator(objs ...runtimeclient.Object) RuntimeClientCreator {
	return &fakeRuntimeClientCreator{objs: objs}
}

func (cc *fakeRuntimeClientCreator) CreateRuntimeClient(kubeCtx string) (*Client, error) {
	b := fake.NewClientBuilder()
	b.WithScheme(scheme.Scheme)
	b.WithObjects(cc.objs...)

	return &Client{
		RuntimeClient: b.Build(),
	}, nil
}
