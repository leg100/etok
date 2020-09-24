package workspace

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"testing"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestNewWorkspace(t *testing.T) {
	//workspace := func(namespace, name string, queue ...string) *v1alpha1.Workspace {
	//	return &v1alpha1.Workspace{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name:      name,
	//			Namespace: namespace,
	//		},
	//		Status: v1alpha1.WorkspaceStatus{
	//			Conditions: status.Conditions{
	//				{
	//					Type:   v1alpha1.ConditionHealthy,
	//					Status: corev1.ConditionTrue,
	//				},
	//			},
	//			Queue: queue,
	//		},
	//	}
	//}

	pod := func(namespace, name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "workspace-" + name,
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				InitContainerStatuses: []corev1.ContainerStatus{
					{
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name     string
		stokObjs []runtime.Object
		kubeObjs []runtime.Object
		args     []string
		code     int
		err      string
	}{
		{
			name:     "WithDefaults",
			kubeObjs: []runtime.Object{pod("default", "xxx")},
			args:     []string{"workspace", "new"},
			code:     1,
			err:      "accepts 1 arg(s), received 0",
		},
		{
			name:     "WithoutFlags",
			kubeObjs: []runtime.Object{pod("default", "foo")},
			args:     []string{"workspace", "new", "foo"},
		},
		{
			name: "WithUserSuppliedSecretAndServiceAccount",
			kubeObjs: []runtime.Object{
				pod("default", "foo"),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bar",
						Namespace: "default",
					},
				},
			},
			args: []string{"workspace", "new", "foo", "--secret", "foo", "--service-account", "bar"},
		},
		{
			name: "WithDefaultSecretAndServiceAccount",
			kubeObjs: []runtime.Object{
				pod("default", "foo"),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "stok",
						Namespace: "default",
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "stok",
						Namespace: "default",
					},
				},
			},
			args: []string{"workspace", "new", "foo"},
		},
		{
			name:     "WithSpecificNamespace",
			kubeObjs: []runtime.Object{pod("foo", "foo")},
			args:     []string{"workspace", "new", "foo", "--namespace", "foo"},
		},
		{
			name:     "WithCacheSettings",
			kubeObjs: []runtime.Object{pod("default", "foo")},
			args:     []string{"workspace", "new", "foo", "--size", "999Gi", "--storage-class", "lumpen-proletariat"},
		},
		{
			name:     "WithContextFlag",
			kubeObjs: []runtime.Object{pod("default", "foo")},
			args:     []string{"workspace", "new", "foo", "--context", "oz-cluster"},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Populate fake kubernetes client with relevant objects
			fakeKubeClient := kfake.NewSimpleClientset(tt.kubeObjs...)
			t.Override(&k8s.KubeClient, func() (kubernetes.Interface, error) {
				return fakeKubeClient, nil
			})

			fakeStokClient := fake.NewSimpleClientset(tt.stokObjs...)
			t.Override(&k8s.StokClient, func() (stokclient.Interface, error) {
				return fakeStokClient, nil
			})

			// Mock call to attach to pod TTY
			t.Override(&k8s.Attach, func(pod *corev1.Pod) error { return nil })

			// Mock call to retrieve pod logs
			t.Override(&k8s.GetLogs, func(ctx context.Context, kc kubernetes.Interface, pod *corev1.Pod, container string) (io.ReadCloser, error) {
				return ioutil.NopCloser(bytes.NewBufferString("test output")), nil
			})

			// Execute cobra command
			out := new(bytes.Buffer)
			code, err := newStokCmd(tt.args, out, out).Execute()

			if tt.err != "" {
				require.EqualError(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.code, code)
		})
	}
}
