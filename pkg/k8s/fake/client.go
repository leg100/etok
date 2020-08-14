package fake

import (
	"context"
	"io"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/leg100/stok/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type client struct {
	runtimeclient.Client
	factory *Factory
	config  *rest.Config
}

func (c *client) NewCache(namespace string) (runtimecache.Cache, error) {
	mapper, err := apiutil.NewDynamicRESTMapper(c.config, apiutil.WithLazyDiscovery)
	if err != nil {
		return nil, err
	}

	return runtimecache.New(c.config, runtimecache.Options{
		Scheme:    scheme.Scheme,
		Mapper:    mapper,
		Namespace: namespace,
	})
}

// No-op attach method to keep tests passing
func (c *client) Attach(pod *corev1.Pod) error {
	return nil
}

func (c *client) GetLogs(namespace, name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("test logs")), nil
}

func (c *client) Get(ctx context.Context, key runtimeclient.ObjectKey, obj runtime.Object) error {
	updatedObj, err := c.factory.getReactors.Apply(c.Client, ctx, key, obj)
	if err != nil {
		return err
	}
	return c.Client.Get(ctx, key, updatedObj)
}

func (c *client) Delete(ctx context.Context, obj runtime.Object, opts ...runtimeclient.DeleteOption) error {
	// We don't even need/use a key for deleting objects, but the reactor function is expecting one.
	// Ignore any error.
	key, _ := runtimeclient.ObjectKeyFromObject(obj)
	updatedObj, err := c.factory.deleteReactors.Apply(c.Client, ctx, key, obj)
	if err != nil {
		return err
	}
	return c.Client.Delete(ctx, updatedObj)
}

func (c *client) Create(ctx context.Context, obj runtime.Object, opts ...runtimeclient.CreateOption) error {
	// We don't even need/use a key for creating objects, but the reactor function is expecting one.
	// Ignore any error.
	key, _ := runtimeclient.ObjectKeyFromObject(obj)
	updatedObj, err := c.factory.createReactors.Apply(c.Client, ctx, key, obj)
	if err != nil {
		return err
	}
	return c.Client.Create(ctx, updatedObj)
}
