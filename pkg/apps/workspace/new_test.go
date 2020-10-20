package workspace

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/controllers"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testcore "k8s.io/client-go/testing"
)

func TestNewWorkspace(t *testing.T) {
	tests := []struct {
		name       string
		err        bool
		workspace  string
		namespace  string
		setOpts    func(*app.Options)
		assertions func(*app.Options)
		mocks      func(*app.Options)
	}{
		{
			name: "defaults",
		},
		{
			name: "do not create secret",
			setOpts: func(opts *app.Options) {
				opts.CreateSecret = false
			},
		},
		{
			name: "do not create service account",
			setOpts: func(opts *app.Options) {
				opts.CreateServiceAccount = false
			},
		},
		{
			name:      "specific name and namespace",
			workspace: "networking",
			namespace: "dev",
		},
		{
			name: "cleanup resources upon error",
			err:  true,
			mocks: func(opts *app.Options) {
				mockError(opts)
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			opts, err := app.NewFakeOptsWithClients(new(bytes.Buffer))
			require.NoError(t, err)

			if tt.setOpts != nil {
				tt.setOpts(opts)
			}

			// Override default if specified
			if tt.workspace != "" {
				opts.Workspace = tt.workspace
			}

			// Override default if specified
			if tt.namespace != "" {
				opts.Namespace = tt.namespace
			}

			mockWorkspaceController(opts)

			if tt.mocks != nil {
				tt.mocks(opts)
			}

			app := NewFromOpts(opts)
			t.CheckError(tt.err, app.Run(context.Background()))

			if !tt.err {
				// Confirm workspace exists
				ws, err := app.(*NewWorkspace).WorkspaceClient.Get(context.Background(), opts.Workspace, metav1.GetOptions{})
				require.NoError(t, err)

				// Confirm secret created if requested
				if opts.CreateSecret {
					_, err := app.(*NewWorkspace).SecretClient.Get(context.Background(), opts.WorkspaceSpec.SecretName, metav1.GetOptions{})
					require.NoError(t, err)
				}

				// Confirm service account created if requested
				if opts.CreateServiceAccount {
					_, err := app.(*NewWorkspace).ServiceAccountClient.Get(context.Background(), opts.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
					require.NoError(t, err)
				}
				// Confirm wait annotation key has been deleted
				assert.False(t, controllers.IsSynchronising(ws))

				/// Confirm env file has been written
				stokenv, err := env.ReadStokEnv(opts.Path)
				require.NoError(t, err)
				assert.Equal(t, opts.Namespace, stokenv.Namespace())
				assert.Equal(t, opts.Workspace, stokenv.Workspace())
			} else {
				// Confirm resources were deleted upon error
				_, err := app.(*NewWorkspace).WorkspaceClient.Get(context.Background(), opts.Workspace, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))

				_, err = app.(*NewWorkspace).SecretClient.Get(context.Background(), opts.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))

				_, err = app.(*NewWorkspace).ServiceAccountClient.Get(context.Background(), opts.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			}

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}

// When a workspace create event occurs create a pod
func mockWorkspaceController(opts *app.Options) {
	createPodAction := func(action testcore.Action) (bool, runtime.Object, error) {
		ws := action.(testcore.CreateAction).GetObject().(*v1alpha1.Workspace)
		pod := testPod(ws.GetNamespace(), ws.GetName())
		opts.KubeClient().CoreV1().Pods(ws.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})

		return false, action.(testcore.CreateAction).GetObject(), nil
	}

	opts.StokClient().(*fake.Clientset).PrependReactor("create", "workspaces", createPodAction)
}

func mockError(opts *app.Options) {
	errAction := func(action testcore.Action) (bool, runtime.Object, error) {
		return true, action.(testcore.CreateAction).GetObject(), fmt.Errorf("fake error")
	}

	opts.StokClient().(*fake.Clientset).PrependReactor("update", "workspaces", errAction)
}

func testPod(namespace, name string) *corev1.Pod {
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
