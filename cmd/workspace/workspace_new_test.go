package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/kr/pty"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/logstreamer"
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
		args       []string
		err        bool
		objs       []runtime.Object
		setOpts    func(*app.Options)
		assertions func(*NewOptions)
	}{
		{
			name: "missing workspace name",
			args: []string{},
			err:  true,
		},
		{
			name: "create workspace",
			args: []string{"foo"},
			assertions: func(o *NewOptions) {
				// Confirm workspace resource has been created
				_, err := o.WorkspacesClient("default").Get(context.Background(), "foo", metav1.GetOptions{})
				require.NoError(t, err)

				/// Confirm env file has been written
				stokenv, err := env.ReadStokEnv(o.Path)
				require.NoError(t, err)
				assert.Equal(t, "default", stokenv.Namespace())
				assert.Equal(t, "foo", stokenv.Workspace())
			},
		},
		{
			name: "create default secret and service account",
			args: []string{"foo"},
			assertions: func(o *NewOptions) {
				_, err := o.SecretsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.NoError(t, err)
				_, err = o.ServiceAccountsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "create custom secret and service account",
			args: []string{"foo", "--service-account", "foo", "--secret", "bar"},
			assertions: func(o *NewOptions) {
				_, err := o.ServiceAccountsClient(o.Namespace).Get(context.Background(), "foo", metav1.GetOptions{})
				assert.NoError(t, err)
				_, err = o.SecretsClient(o.Namespace).Get(context.Background(), "bar", metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "do not create secret",
			args: []string{"foo", "--no-create-secret"},
			assertions: func(o *NewOptions) {
				_, err := o.SecretsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			},
		},
		{
			name: "do not create service account",
			args: []string{"foo", "--no-create-service-account"},
			assertions: func(o *NewOptions) {
				_, err := o.ServiceAccountsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			},
		},
		{
			name: "non-default namespace",
			args: []string{"foo", "--namespace", "bar"},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "bar", o.Namespace)
			},
		},
		{
			name: "cleanup resources upon error",
			args: []string{"foo"},
			err:  true,
			setOpts: func(o *app.Options) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fmt.Errorf("fake error")
				}
			},
			assertions: func(o *NewOptions) {
				_, err := o.WorkspacesClient(o.Namespace).Get(context.Background(), o.Workspace, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))

				_, err = o.SecretsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))

				_, err = o.ServiceAccountsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			},
		},
		{
			name: "do not cleanup resources upon error",
			args: []string{"foo", "--no-cleanup"},
			err:  true,
			setOpts: func(o *app.Options) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fmt.Errorf("fake error")
				}
			},
			assertions: func(o *NewOptions) {
				_, err := o.WorkspacesClient(o.Namespace).Get(context.Background(), o.Workspace, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.SecretsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ServiceAccountsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "with existing custom secret and service account",
			args: []string{"foo", "--secret", "foo", "--service-account", "bar"},
			objs: []runtime.Object{
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
		},
		{
			name: "with cache settings",
			args: []string{"foo", "--size", "999Gi", "--storage-class", "lumpen-proletariat"},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "999Gi", o.WorkspaceSpec.Cache.Size)
				assert.Equal(t, "lumpen-proletariat", o.WorkspaceSpec.Cache.StorageClass)
			},
		},
		{
			name: "with kube context flag",
			args: []string{"foo", "--context", "oz-cluster"},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "oz-cluster", o.KubeContext)
			},
		},
		{
			name: "debug flag",
			args: []string{"foo", "--debug"},
			assertions: func(o *NewOptions) {
				ws, err := o.WorkspacesClient(o.Namespace).Get(context.Background(), o.Workspace, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, true, ws.GetDebug())
			},
		},
		{
			name: "log stream output",
			args: []string{"foo"},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())
			},
		},
		{
			name: "attach",
			args: []string{"foo"},
			setOpts: func(o *app.Options) {
				// Create pseudoterminal slave to trigger tty detection
				_, pts, err := pty.Open()
				require.NoError(t, err)
				o.In = pts
			},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "fake attach", o.Out.(*bytes.Buffer).String())
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			opts, err := app.NewFakeOpts(out, tt.objs...)
			require.NoError(t, err)

			if tt.setOpts != nil {
				tt.setOpts(opts)
			}

			cmd, cmdOpts := NewCmd(opts)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			// Override path
			path := t.NewTempDir().Chdir().Root()
			cmdOpts.Path = path

			mockWorkspaceController(opts, cmdOpts)

			// Set debug flag (that root cmd otherwise sets)
			cmd.Flags().BoolVar(&opts.Debug, "debug", false, "debug flag")

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			if tt.assertions != nil {
				tt.assertions(cmdOpts)
			}
		})
	}
}

// When a workspace create event occurs create a pod
func mockWorkspaceController(opts *app.Options, o *NewOptions) {
	createPodAction := func(action testcore.Action) (bool, runtime.Object, error) {
		ws := action.(testcore.CreateAction).GetObject().(*v1alpha1.Workspace)
		pod := testPod(ws.GetNamespace(), ws.GetName())
		o.PodsClient(ws.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})

		return false, action.(testcore.CreateAction).GetObject(), nil
	}

	opts.ClientCreator.(*client.FakeClientCreator).PrependReactor("create", "workspaces", createPodAction)
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
