package fake

import (
	"context"
	"io"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type client struct {
	runtimeclient.Client
	factory *Factory
	config  *rest.Config
}

// No-op attach method to keep tests passing
func (c *client) Attach(pod *corev1.Pod) error {
	return nil
}

func (c *client) GetLogs(namespace, name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("test logs")), nil
}

// No-op create
func (c *client) Create(ctx context.Context, obj runtime.Object, opts ...runtimeclient.CreateOption) error {
	return nil
}

// No-op update
func (c *client) Update(ctx context.Context, obj runtime.Object, opts ...runtimeclient.UpdateOption) error {
	return nil
}
