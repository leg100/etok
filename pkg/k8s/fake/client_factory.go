package fake

import (
	"context"
	"io"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	runtimefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type Factory struct {
	Objs    []runtime.Object
	Client  runtimeclient.Client
	Gets    int
	Context string
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

// No-op attach method to keep tests passing
func (c *client) Attach(pod *corev1.Pod) error {
	return nil
}

func (c *client) GetLogs(namespace, name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("test logs")), nil
}

func (c *client) Get(ctx context.Context, key runtimeclient.ObjectKey, obj runtime.Object) error {
	c.factory.Gets++
	return c.Client.Get(ctx, key, obj)
}

func (c *client) Create(ctx context.Context, obj runtime.Object, opts ...runtimeclient.CreateOption) error {
	switch t := obj.(type) {
	case command.Interface:
		err := c.create(ctx, t, opts...)

		// Mock command controller; t should now have a generated name for pod to use
		if err := c.createPod(t.GetNamespace(), t.GetName()); err != nil {
			return err
		}

		// Callee is expecting the create command error obj, so return that
		return err
	case *v1alpha1.Workspace:
		err := c.create(ctx, t, opts...)

		// Mock workspace controller
		if err := c.createWorkspacePod(t.GetNamespace(), t.PodName()); err != nil {
			return err
		}

		t.Status.Conditions.SetCondition(status.Condition{
			Type:   v1alpha1.ConditionHealthy,
			Status: corev1.ConditionTrue,
		})

		// Callee is expecting the create workspace error obj, so return that
		return err
	default:
		return c.create(ctx, t, opts...)
	}
}

func (c *client) createPod(namespace, name string) error {
	var pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	return c.create(context.TODO(), pod)
}

func (c *client) createWorkspacePod(namespace, name string) error {
	var pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	return c.create(context.TODO(), pod)
}

func (c *client) create(ctx context.Context, obj runtime.Object, opts ...runtimeclient.CreateOption) error {
	c.factory.Objs = append(c.factory.Objs, obj)
	return c.Client.Create(ctx, obj, opts...)
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
