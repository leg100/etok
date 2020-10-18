package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewWorkspace(t *testing.T) {
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
		name       string
		objs       []runtime.Object
		args       []string
		err        bool
		assertions func(opts *app.Options)
	}{
		{
			name: "WithDefaults",
			objs: []runtime.Object{pod("default", "xxx")},
			args: []string{"workspace", "new"},
			err:  true,
		},
		{
			name: "WithoutFlags",
			objs: []runtime.Object{pod("default", "foo")},
			args: []string{"workspace", "new", "foo"},
		},
		{
			name: "WithUserSuppliedSecretAndServiceAccount",
			objs: []runtime.Object{
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
			args: []string{"workspace", "new", "foo"},
		},
		{
			name: "WithSpecificNamespace",
			objs: []runtime.Object{pod("foo", "foo")},
			args: []string{"workspace", "new", "foo", "--namespace", "foo"},
			assertions: func(opts *app.Options) {
				assert.Equal(t, "foo", opts.Workspace)
				assert.Equal(t, "foo", opts.Namespace)
			},
		},
		{
			name: "WithCacheSettings",
			objs: []runtime.Object{pod("default", "foo")},
			args: []string{"workspace", "new", "foo", "--size", "999Gi", "--storage-class", "lumpen-proletariat"},
			assertions: func(opts *app.Options) {
				assert.Equal(t, "foo", opts.Workspace)
				assert.Equal(t, "999Gi", opts.WorkspaceSpec.Cache.Size)
				assert.Equal(t, "lumpen-proletariat", opts.WorkspaceSpec.Cache.StorageClass)
			},
		},
		{
			name: "WithContextFlag",
			args: []string{"workspace", "new", "foo", "--context", "oz-cluster"},
			assertions: func(opts *app.Options) {
				assert.Equal(t, "oz-cluster", opts.KubeContext)
			},
		},
		{
			name: "ensure kube clients are created",
			args: []string{"workspace", "new", "foo"},
			assertions: func(opts *app.Options) {
				assert.NotNil(t, opts.KubeClient())
				assert.NotNil(t, opts.StokClient())
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			opts, err := app.NewFakeOpts(new(bytes.Buffer), tt.objs...)
			require.NoError(t, err)

			t.CheckError(tt.err, ParseArgs(context.Background(), tt.args, opts))

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}
