package client

import (
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	sfake "github.com/leg100/etok/pkg/k8s/etokclient/fake"
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
	var kubeObjs, etokObjs []runtime.Object
	for _, obj := range f.objs {
		switch obj.(type) {
		case *v1alpha1.Run, *v1alpha1.Workspace:
			etokObjs = append(etokObjs, obj)
		default:
			kubeObjs = append(kubeObjs, obj)
		}
	}

	EtokClient := sfake.NewSimpleClientset(etokObjs...)
	for _, r := range f.reactors {
		EtokClient.PrependReactor(r.Verb, r.Resource, r.Reaction)
	}

	return &Client{
		Config:     &rest.Config{},
		EtokClient: EtokClient,
		KubeClient: kfake.NewSimpleClientset(kubeObjs...),
	}, nil
}

// Add a reactor to the list of reactors to be prepended.
func (f *FakeClientCreator) PrependReactor(verb, resource string, reaction testing.ReactionFunc) {
	f.reactors = append(f.reactors, testing.SimpleReactor{verb, resource, reaction})
}
