package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/podhandler"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestPodConnect(t *testing.T) {
	tests := []struct {
		name       string
		err        bool
		assertions func(opts *app.Options)
		ph podhandler.Interface
		out string
	}{
		{
			name: "successful attach",
			ph: &testAttachSucceed{},
			out: "fake attach",
		},
		{
			name: "fallback to logs",
			ph: &testAttachFail{},
			out: "fake logs",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			opts, err := app.NewFakeOptsWithClients(out)
			require.NoError(t, err)

			// Create pod
			pod, err := opts.KubeClient().
				CoreV1().
				Pods(opts.Namespace).
				Create(context.Background(), testPod(opts.Namespace, opts.Name), metav1.CreateOptions{})
			require.NoError(t, err)

			err = PodConnect(
				context.Background(),
				tt.ph,
				opts.KubeClient(),
				opts.KubeConfig,
				pod,
				opts.Out,
				func() error { return nil },
			)
			assert.NoError(t, err)

			assert.Regexp(t, tt.out, out)
		})
	}
}

func testPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

type testAttachSucceed struct {}

func (h *testAttachSucceed) Attach(cfg *rest.Config, pod *corev1.Pod, out io.Writer) error {
	fmt.Fprintln(out, "fake attach")
	return nil
}

func (h *testAttachSucceed) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
}

type testAttachFail struct {}

func (h *testAttachFail) Attach(cfg *rest.Config, pod *corev1.Pod, out io.Writer) error {
	return fmt.Errorf("fake error")
}

func (h *testAttachFail) GetLogs(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewBufferString("fake logs")), nil
}
