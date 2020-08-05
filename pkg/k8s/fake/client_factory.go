package fake

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/leg100/stok/pkg/k8s"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type Factory struct {
	Objs           []runtime.Object
	Client         runtimeclient.Client
	Context        string
	getReactors    Reactors
	createReactors Reactors
	deleteReactors Reactors
}

var _ k8s.FactoryInterface = &Factory{}

func NewFactory(objs ...runtime.Object) *Factory {
	return &Factory{Objs: objs}
}

type client struct {
	runtimeclient.Client
	factory *Factory
}

func (f *Factory) NewClient(s *runtime.Scheme, context string) (k8s.Client, error) {
	rc := runtimefake.NewFakeClientWithScheme(s, f.Objs...)
	f.Client = rc
	f.Context = context

	return &client{factory: f, Client: rc}, nil
}

func (f *Factory) AddReactor(action string, reactor Reactor) *Factory {
	switch action {
	case "create":
		f.createReactors = append(f.createReactors, reactor)
	case "delete":
		f.deleteReactors = append(f.deleteReactors, reactor)
	case "get":
		f.getReactors = append(f.getReactors, reactor)
	default:
		panic(fmt.Sprintf("no reactor support for %s", action))
	}
	return f
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

// A really naff fake rest client. Any requests matching the given namespace and resource type will
// receive a response containing a single item list, containing the first obj stored in the factory. If
// the namespace or resource type does not match, or there are no objs in the factory, then a empty
// list is returned.
// Stok CLI only uses the rest client to watch for an obj, and only one obj, so this should suffice for
// testing purposes.
// func (f *Factory) RESTClientForGVK(gvk schema.GroupVersionKind, _ *rest.Config, s *runtime.Scheme) (rest.Interface, error) {
// 	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
// 	ns := scheme.Codecs.WithoutConversion()
//
// 	path := fmt.Sprintf("/namespaces/%s/%s", f.watchNamespace, f.watchResource)
//
// 	return &fake.RESTClient{
// 		GroupVersion:         gvk.GroupVersion(),
// 		NegotiatedSerializer: ns,
// 		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
// 			if req.URL.Path == path && f.watchObj != nil {
// 				fmt.Printf("%#v\n", f.watchObj)
// 				return &http.Response{
// 					StatusCode: http.StatusOK,
// 					Header:     cmdtesting.DefaultHeader(),
// 					Body:       cmdtesting.ObjBody(codec, f.watchObj),
// 				}, nil
// 			}
// 			return &http.Response{
// 				StatusCode: http.StatusOK,
// 				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte("{}"))),
// 			}, nil
// 		}),
// 	}, nil
// }
