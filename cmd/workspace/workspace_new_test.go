package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/creack/pty"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/logstreamer"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewWorkspace(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		err        bool
		objs       []runtime.Object
		setOpts    func(*cmdutil.Options)
		assertions func(*NewOptions)
	}{
		{
			name: "missing workspace name",
			args: []string{},
			err:  true,
		},
		{
			name: "create workspace",
			args: []string{"default/foo"},
			objs: []runtime.Object{testPod("foo")},
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
			args: []string{"default/foo"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				_, err := o.SecretsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.NoError(t, err)
				_, err = o.ServiceAccountsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "create custom secret and service account",
			args: []string{"default/foo", "--service-account", "foo", "--secret", "bar"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				_, err := o.ServiceAccountsClient(o.Namespace).Get(context.Background(), "foo", metav1.GetOptions{})
				assert.NoError(t, err)
				_, err = o.SecretsClient(o.Namespace).Get(context.Background(), "bar", metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "do not create secret",
			args: []string{"default/foo", "--no-create-secret"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				_, err := o.SecretsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.SecretName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			},
		},
		{
			name: "do not create service account",
			args: []string{"default/foo", "--no-create-service-account"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				_, err := o.ServiceAccountsClient(o.Namespace).Get(context.Background(), o.WorkspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			},
		},
		{
			name: "non-default namespace",
			args: []string{"bar/foo"},
			objs: []runtime.Object{testPod("foo", namespace("bar"))},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "bar", o.Namespace)
			},
		},
		{
			name: "cleanup resources upon error",
			args: []string{"default/foo"},
			objs: []runtime.Object{testPod("foo")},
			err:  true,
			setOpts: func(o *cmdutil.Options) {
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
			args: []string{"default/foo", "--no-cleanup"},
			objs: []runtime.Object{testPod("foo")},
			err:  true,
			setOpts: func(o *cmdutil.Options) {
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
			args: []string{"default/foo", "--secret", "foo", "--service-account", "bar"},
			objs: []runtime.Object{
				testPod("foo"),
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
			args: []string{"default/foo", "--size", "999Gi", "--storage-class", "lumpen-proletariat"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "999Gi", o.WorkspaceSpec.Cache.Size)
				assert.Equal(t, "lumpen-proletariat", o.WorkspaceSpec.Cache.StorageClass)
			},
		},
		{
			name: "with kube context flag",
			args: []string{"default/foo", "--context", "oz-cluster"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "oz-cluster", o.KubeContext)
			},
		},
		{
			name: "debug flag",
			args: []string{"default/foo", "--debug"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				ws, err := o.WorkspacesClient(o.Namespace).Get(context.Background(), o.Workspace, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, true, ws.GetDebug())
			},
		},
		{
			name: "log stream output",
			args: []string{"default/foo"},
			objs: []runtime.Object{testPod("foo")},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())
			},
		},
		{
			name: "attach",
			args: []string{"default/foo"},
			objs: []runtime.Object{testPod("foo")},
			setOpts: func(o *cmdutil.Options) {
				// Create pseudoterminal slave to trigger tty detection
				_, pts, err := pty.Open()
				require.NoError(t, err)
				o.In = pts
			},
			assertions: func(o *NewOptions) {
				assert.Equal(t, "fake attach", o.Out.(*bytes.Buffer).String())
			},
		},
		{
			name: "disable tty",
			args: []string{"default/foo", "--no-tty"},
			objs: []runtime.Object{testPod("foo")},
			setOpts: func(o *cmdutil.Options) {
				// Ensure tty is overridden
				_, pts, err := pty.Open()
				require.NoError(t, err)
				o.In = pts
			},
			assertions: func(o *NewOptions) {
				// With tty disabled, it should stream logs not attach
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())
			},
		},
		{
			name: "non-zero exit code",
			args: []string{"default/foo"},
			objs: []runtime.Object{testPod("foo", exitCode(5))},
			err:  true,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			opts, err := cmdutil.NewFakeOpts(out, tt.objs...)
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

			// Set debug flag (that root cmd otherwise sets)
			cmd.Flags().BoolVar(&opts.Debug, "debug", false, "debug flag")

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			if tt.assertions != nil {
				tt.assertions(cmdOpts)
			}
		})
	}
}

func testPod(name string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.WorkspacePodName(name),
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					// NOTE: The pod is both running and terminated in order to pass tests. The
					// alternative is to use a complicated set of reactors, which are known not to
					// play well with k8s informers:
					// https://github.com/kubernetes/kubernetes/pull/95897
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			},
		},
	}
	for _, option := range opts {
		option(pod)
	}
	return pod
}

func namespace(ns string) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.SetNamespace(ns)
	}
}

func phase(phase corev1.PodPhase) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Status.Phase = phase
	}
}

func exitCode(code int32) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		pod.Status.InitContainerStatuses[0].State.Terminated.ExitCode = code
	}
}
