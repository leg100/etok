package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	sfake "github.com/leg100/stok/pkg/k8s/stokclient/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

type fake struct {
	// Kubernetes built-in client
	kubeClient kubernetes.Interface

	// Stok generated client
	stokClient stokclient.Interface

	// Fake objs
	objs []runtime.Object
}

func NewFakeClient(objs ...runtime.Object) Client {
	return &fake{objs: objs}
}

func (f *fake) KubeConfig() *rest.Config {
	return nil
}

func (f *fake) KubeClient() kubernetes.Interface {
	return f.kubeClient
}

func (f *fake) StokClient() stokclient.Interface {
	return f.stokClient
}

func (f *fake) Create(kubeCtx string) error {
	var kubeObjs, stokObjs []runtime.Object
	for _, obj := range f.objs {
		switch obj.(type) {
		case *v1alpha1.Run, *v1alpha1.Workspace:
			stokObjs = append(stokObjs, obj)
		default:
			kubeObjs = append(kubeObjs, obj)
		}
	}

	f.stokClient = sfake.NewSimpleClientset(stokObjs...)
	f.kubeClient = kfake.NewSimpleClientset(kubeObjs...)
	return nil
}

// Rather poor fake - pass in wrong container name to trigger error
func (f *fake) GetLogs(ctx context.Context, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	if container == "runner" {
		return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
	} else {
		return nil, fmt.Errorf("wrong container name")
	}
}

// Terrible fake
func (f *fake) Attach(pod *corev1.Pod, container, magicString string, in *os.File, out io.Writer) error {
	out.Write([]byte("fake attach"))
	return nil
}
