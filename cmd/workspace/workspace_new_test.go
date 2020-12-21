package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	etokerrors "github.com/leg100/etok/pkg/errors"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewWorkspace(t *testing.T) {
	var fakeError = errors.New("fake error")

	tests := []struct {
		name string
		args []string
		err  func(*testutil.T, error)
		// Toggle mocking a successful reconcile status
		disableMockReconcile bool
		objs                 []runtime.Object
		setOpts              func(*cmdutil.Options)
		assertions           func(*newOptions)
	}{
		{
			name: "missing workspace name",
			args: []string{},
			err: func(t *testutil.T, err error) {
				assert.Equal(t, err.Error(), "accepts 1 arg(s), received 0")
			},
		},
		{
			name: "create workspace",
			args: []string{"foo"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				// Confirm workspace resource has been created
				_, err := o.WorkspacesClient("default").Get(context.Background(), "foo", metav1.GetOptions{})
				require.NoError(t, err)

				/// Confirm env file has been written
				etokenv, err := env.Read(o.path)
				require.NoError(t, err)
				assert.Equal(t, "default", etokenv.Namespace)
				assert.Equal(t, "foo", etokenv.Workspace)
			},
		},
		{
			name: "create default secret and service account",
			args: []string{"foo"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				_, err := o.SecretsClient(o.namespace).Get(context.Background(), o.workspaceSpec.SecretName, metav1.GetOptions{})
				assert.NoError(t, err)
				_, err = o.ServiceAccountsClient(o.namespace).Get(context.Background(), o.workspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "create custom secret and service account",
			args: []string{"foo", "--service-account", "foo", "--secret", "bar"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				_, err := o.ServiceAccountsClient(o.namespace).Get(context.Background(), "foo", metav1.GetOptions{})
				assert.NoError(t, err)
				_, err = o.SecretsClient(o.namespace).Get(context.Background(), "bar", metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "do not create secret",
			args: []string{"foo", "--no-create-secret"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				_, err := o.SecretsClient(o.namespace).Get(context.Background(), o.workspaceSpec.SecretName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))
			},
		},
		{
			name: "do not create service account",
			args: []string{"foo", "--no-create-service-account"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				_, err := o.ServiceAccountsClient(o.namespace).Get(context.Background(), o.workspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))
			},
		},
		{
			name: "non-default namespace",
			args: []string{"foo", "--namespace", "bar"},
			objs: []runtime.Object{testobj.WorkspacePod("bar", "foo")},
			assertions: func(o *newOptions) {
				assert.Equal(t, "bar", o.namespace)
			},
		},
		{
			name: "cleanup resources upon error",
			args: []string{"foo"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			err: func(t *testutil.T, err error) {
				assert.Equal(t, fakeError, err)
			},
			setOpts: func(o *cmdutil.Options) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fmt.Errorf("fake error")
				}
			},
			assertions: func(o *newOptions) {
				_, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))

				_, err = o.SecretsClient(o.namespace).Get(context.Background(), o.workspaceSpec.SecretName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))

				_, err = o.ServiceAccountsClient(o.namespace).Get(context.Background(), o.workspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))
			},
		},
		{
			name: "do not cleanup resources upon error",
			args: []string{"foo", "--no-cleanup"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			err: func(t *testutil.T, err error) {
				assert.Equal(t, fakeError, err)
			},
			setOpts: func(o *cmdutil.Options) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fmt.Errorf("fake error")
				}
			},
			assertions: func(o *newOptions) {
				_, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.SecretsClient(o.namespace).Get(context.Background(), o.workspaceSpec.SecretName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ServiceAccountsClient(o.namespace).Get(context.Background(), o.workspaceSpec.ServiceAccountName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "with existing custom secret and service account",
			args: []string{"foo", "--secret", "foo", "--service-account", "bar"},
			objs: []runtime.Object{
				testobj.WorkspacePod("default", "foo"),
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
			name: "default storage class is nil",
			args: []string{"foo"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				// Get workspace
				ws, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Empty(t, ws.Spec.Cache.StorageClass)
			},
		},
		{
			name: "explicitly set storage class to empty string",
			args: []string{"foo", "--storage-class", ""},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				// Get workspace
				ws, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, "", *ws.Spec.Cache.StorageClass)
			},
		},
		{
			name: "with cache settings",
			args: []string{"foo", "--size", "999Gi", "--storage-class", "lumpen-proletariat"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				// Get workspace
				ws, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, "999Gi", ws.Spec.Cache.Size)
				assert.Equal(t, "lumpen-proletariat", *ws.Spec.Cache.StorageClass)
			},
		},
		{
			name: "with kube context flag",
			args: []string{"foo", "--context", "oz-cluster"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				assert.Equal(t, "oz-cluster", o.kubeContext)
			},
		},
		{
			name: "log stream output",
			args: []string{"foo"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())
			},
		},
		{
			name: "non-zero exit code",
			args: []string{"foo"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo", testobj.WithInstallerExitCode(5))},
			err: func(t *testutil.T, err error) {
				// want exit code 5
				var exiterr etokerrors.ExitError
				if assert.True(t, errors.As(err, &exiterr)) {
					assert.Equal(t, 5, exiterr.ExitCode())
				}
			},
		},
		{
			name: "set terraform version",
			args: []string{"foo", "--terraform-version", "0.12.17"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				// Get workspace
				ws, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, "0.12.17", ws.Spec.TerraformVersion)
			},
		},
		{
			name: "set privileged commands",
			args: []string{"foo", "--privileged-commands", "apply,destroy,sh"},
			objs: []runtime.Object{testobj.WorkspacePod("default", "foo")},
			assertions: func(o *newOptions) {
				// Get workspace
				ws, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, []string{"apply", "destroy", "sh"}, ws.Spec.PrivilegedCommands)
			},
		},
		{
			name:                 "reconcile timeout exceeded",
			args:                 []string{"foo", "--reconcile-timeout", "10ms"},
			disableMockReconcile: true,
			err: func(t *testutil.T, err error) {
				assert.True(t, errors.Is(err, errReconcileTimeout))
			},
		},
		{
			name: "pod timeout exceeded",
			args: []string{"foo", "--pod-timeout", "10ms"},
			// Deliberately omit pod
			objs: []runtime.Object{},
			err: func(t *testutil.T, err error) {
				assert.True(t, errors.Is(err, errPodTimeout))
			},
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

			cmd, cmdOpts := newCmd(opts)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			// Override path
			path := t.NewTempDir().Chdir().Root()
			cmdOpts.path = path

			if !tt.disableMockReconcile {
				// Mock successful reconcile
				cmdOpts.reconciled = true
			}

			err = cmd.ExecuteContext(context.Background())
			if tt.err != nil {
				tt.err(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.assertions != nil {
				tt.assertions(cmdOpts)
			}
		})
	}
}
