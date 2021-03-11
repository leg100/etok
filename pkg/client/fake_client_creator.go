package client

import (
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	sfake "github.com/leg100/etok/pkg/k8s/etokclient/fake"
	"github.com/leg100/etok/pkg/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Implements ClientCreator
type FakeClientCreator struct {
	// Fake objs
	objs     []runtime.Object
	reactors []testing.SimpleReactor
}

func NewFakeClientCreator(objs ...runtime.Object) ClientCreator {
	return &FakeClientCreator{objs: objs}
}

func (f *FakeClientCreator) Create(kubeCtx string) (*Client, error) {
	var kubeObjs, etokObjs []runtime.Object
	for _, obj := range f.objs {
		switch obj.(type) {
		case *v1alpha1.Run, *v1alpha1.Workspace:
			etokObjs = append(etokObjs, obj)
		default:
			kubeObjs = append(kubeObjs, obj)
		}
	}

	etokClient := sfake.NewSimpleClientset(etokObjs...)
	for _, r := range f.reactors {
		etokClient.PrependReactor(r.Verb, r.Resource, r.Reaction)
	}

	return &Client{
		Config:        &rest.Config{},
		EtokClient:    etokClient,
		KubeClient:    kfake.NewSimpleClientset(kubeObjs...),
		RuntimeClient: fake.NewFakeClientWithScheme(scheme.Scheme, f.objs...),
	}, nil
}
