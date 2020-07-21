package k8s

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubectl/pkg/scheme"
)

func TestWaitUntil(t *testing.T) {
	pods := &corev1.PodList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "15",
		},
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "my-pod",
					Namespace:       "test",
					ResourceVersion: "18",
				},
			},
		},
	}

	codec := scheme.Codecs.LegacyCodec(scheme.Scheme.PrioritizedVersionsAllGroups()...)
	rc := &fake.RESTClient{
		GroupVersion:         corev1.SchemeGroupVersion,
		NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/namespaces/test/pods" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     cmdtesting.DefaultHeader(),
					Body:       cmdtesting.ObjBody(codec, pods),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewBuffer([]byte("{}"))),
			}, nil
		}),
	}

	exitCondition := func(event watch.Event) (bool, error) {
		return true, nil
	}

	result, err := WaitUntil(rc, &corev1.Pod{}, "my-pod", "test", "pods", exitCondition, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, "my-pod", result.(*corev1.Pod).GetName())
}
