package client

import (
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	sfake "github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"k8s.io/apimachinery/pkg/runtime"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"
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
	var kubeObjs, stokObjs []runtime.Object
	for _, obj := range f.objs {
		switch obj.(type) {
		case *v1alpha1.Run, *v1alpha1.Workspace:
			stokObjs = append(stokObjs, obj)
		default:
			kubeObjs = append(kubeObjs, obj)
		}
	}

	stokClient := sfake.NewSimpleClientset(stokObjs...)
	for _, r := range f.reactors {
		stokClient.PrependReactor(r.Verb, r.Resource, r.Reaction)
	}

	return &Client{
		Config:     &rest.Config{},
		StokClient: stokClient,
		KubeClient: kfake.NewSimpleClientset(kubeObjs...),
	}, nil
}

// Add a reactor to the list of reactors to be prepended.
func (f *FakeClientCreator) PrependReactor(verb, resource string, reaction testing.ReactionFunc) {
	f.reactors = append(f.reactors, testing.SimpleReactor{verb, resource, reaction})
}
