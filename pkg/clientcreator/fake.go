package clientcreator

import (
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

type Fake struct {
	// Kubernetes built-in client
	kubeClient kubernetes.Interface

	// Stok generated client
	stokClient stokclient.Interface

	// Fake objs
	objs []runtime.Object

	Interface
}

func NewFakeClientCreator(objs ...runtime.Object) *Fake {
	return &Fake{objs: objs}
}

func (f *Fake) KubeConfig() *rest.Config {
	return nil
}

func (f *Fake) KubeClient() kubernetes.Interface {
	return f.kubeClient
}

func (f *Fake) StokClient() stokclient.Interface {
	return f.stokClient
}

func (f *Fake) CreateClients(kubeCtx string) error {
	var kubeObjs, stokObjs []runtime.Object
	for _, obj := range f.objs {
		switch obj.(type) {
		case *v1alpha1.Run, *v1alpha1.Workspace:
			stokObjs = append(stokObjs, obj)
		default:
			kubeObjs = append(kubeObjs, obj)
		}
	}

	f.stokClient = fake.NewSimpleClientset(stokObjs...)
	f.kubeClient = kfake.NewSimpleClientset(kubeObjs...)
	return nil
}
