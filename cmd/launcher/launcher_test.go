package launcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/creack/pty"
	"github.com/go-git/go-git/v5"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/env"
	etokerrors "github.com/leg100/etok/pkg/errors"
	"github.com/leg100/etok/pkg/handlers"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestLauncher(t *testing.T) {
	var fakeError = errors.New("fake error")

	tests := []struct {
		name string
		args []string
		env  *env.Env
		err  error
		objs []runtime.Object
		// Override default command "plan"
		cmd string
		// Size of content to be archived
		size int
		// Mock exit code of runner container
		code int32
		// Override run status
		overrideStatus   func(*v1alpha1.RunStatus)
		factoryOverrides func(*cmdutil.Factory)
		assertions       func(*launcherOptions)
	}{
		{
			name: "plan",
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "default", o.namespace)
				assert.Equal(t, "default", o.workspace)
			},
		},
		{
			name: "queueable commands",
			cmd:  "apply",
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "default", o.namespace)
				assert.Equal(t, "default", o.workspace)
			},
		},
		{
			name: "specific namespace and workspace",
			env:  &env.Env{Namespace: "foo", Workspace: "bar"},
			objs: []runtime.Object{testobj.Workspace("foo", "bar", testobj.WithCombinedQueue("run-12345"))},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "foo", o.namespace)
				assert.Equal(t, "bar", o.workspace)
			},
		},
		{
			name: "namespace flag overrides environment",
			args: []string{"--namespace", "foo"},
			objs: []runtime.Object{testobj.Workspace("foo", "default", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "foo", o.namespace)
				assert.Equal(t, "default", o.workspace)
			},
		},
		{
			name: "workspace flag overrides environment",
			args: []string{"--workspace", "bar"},
			objs: []runtime.Object{testobj.Workspace("default", "bar", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "default", o.namespace)
				assert.Equal(t, "bar", o.workspace)
			},
		},
		{
			name: "arbitrary terraform flag",
			args: []string{"--", "-input", "false"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, []string{"-input", "false"}, o.args)
			},
		},
		{
			name: "context flag",
			args: []string{"--context", "oz-cluster"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "oz-cluster", o.kubeContext)
			},
		},
		{
			name: "approved",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"), testobj.WithPrivilegedCommands("plan"))},
			assertions: func(o *launcherOptions) {
				// Get run
				run, err := o.RunsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				require.NoError(t, err)
				// Get workspace
				ws, err := o.WorkspacesClient(o.namespace).Get(context.Background(), o.workspace, metav1.GetOptions{})
				require.NoError(t, err)
				// Check run's approval annotation is set on workspace
				assert.Equal(t, true, ws.IsRunApproved(run))
			},
		},
		{
			name: "without env file",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			assertions: func(o *launcherOptions) {
				assert.Equal(t, "default", o.namespace)
				assert.Equal(t, "default", o.workspace)
			},
		},
		{
			name: "workspace does not exist",
			err:  errWorkspaceNotFound,
		},
		{
			name: "cleanup resources upon error",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			err:  fakeError,
			factoryOverrides: func(f *cmdutil.Factory) {
				f.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fakeError
				}
			},
			assertions: func(o *launcherOptions) {
				_, err := o.RunsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))

				_, err = o.ConfigMapsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))
			},
		},
		{
			name: "disable cleanup resources upon error",
			args: []string{"--no-cleanup"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			err:  fakeError,
			factoryOverrides: func(f *cmdutil.Factory) {
				f.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fakeError
				}
			},
			assertions: func(o *launcherOptions) {
				_, err := o.RunsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ConfigMapsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "resources are not cleaned up upon exit code error",
			args: []string{},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			overrideStatus: func(status *v1alpha1.RunStatus) {
				// Mock pod returning exit code 5
				var code = 5
				status.ExitCode = &code
			},
			// Expect exit error with exit code 5
			err: etokerrors.NewExitError(5),
			assertions: func(o *launcherOptions) {
				_, err := o.RunsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ConfigMapsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "with tty",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			factoryOverrides: func(opts *cmdutil.Factory) {
				var err error
				opts.In, _, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(o *launcherOptions) {
				// With a tty, launcher should attach not stream logs
				assert.Equal(t, "fake attach", o.Out.(*bytes.Buffer).String())

				// Get run
				run, err := o.RunsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				require.NoError(t, err)
				// With a tty, a handshake is required
				assert.True(t, run.Handshake)
			},
		},
		{
			name: "disable tty",
			args: []string{"--no-tty"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			factoryOverrides: func(f *cmdutil.Factory) {
				// Ensure tty is overridden
				var err error
				_, f.In, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(o *launcherOptions) {
				// With tty disabled, launcher should stream logs not attach
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())

				// Get run
				run, err := o.RunsClient(o.namespace).Get(context.Background(), o.runName, metav1.GetOptions{})
				require.NoError(t, err)
				// With tty disabled, there should be no handshake
				assert.False(t, run.Handshake)
			},
		},
		{
			name: "pod completed with no tty",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
		},
		{
			name: "pod completed with tty",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			factoryOverrides: func(f *cmdutil.Factory) {
				var err error
				_, f.In, err = pty.Open()
				require.NoError(t, err)
			},
			overrideStatus: func(status *v1alpha1.RunStatus) {
				// Mock run having cmpleted
				status.Conditions = []metav1.Condition{
					{
						Type:   v1alpha1.RunCompleteCondition,
						Status: metav1.ConditionTrue,
						Reason: v1alpha1.PodSucceededReason,
					},
				}
			},
			err: handlers.PrematurelySucceededPodError,
		},
		{
			name: "config too big",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			size: 1024*1024 + 1,
			err:  archive.MaxSizeError(archive.MaxConfigSize),
		},
		{
			name: "reconcile timeout exceeded",
			args: []string{"--reconcile-timeout", "10ms"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			overrideStatus: func(status *v1alpha1.RunStatus) {
				// Triggers reconcile timeout
				status.Phase = ""
			},
			err: errReconcileTimeout,
		},
		{
			name: "run failed",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			overrideStatus: func(status *v1alpha1.RunStatus) {
				status.Conditions = []metav1.Condition{
					{
						Type:    v1alpha1.RunFailedCondition,
						Status:  metav1.ConditionTrue,
						Reason:  v1alpha1.FailureReason,
						Message: "mock failure message",
					},
				}
			},
			assertions: func(o *launcherOptions) {
				assert.Contains(t, o.Out.(*bytes.Buffer).String(), "Error: run failed: mock failure message")
			},
			err: handlers.ErrRunFailed,
		},
	}

	// Run tests for each command
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			path := t.NewTempDir().Chdir().WriteRandomFile("test.bin", tt.size).Root()

			// Make the path a git repo unless test specifies otherwise
			_, err := git.PlainInit(path, false)
			require.NoError(t, err)

			// Write .terraform/environment
			if tt.env != nil {
				require.NoError(t, tt.env.Write(path))
			}

			out := new(bytes.Buffer)
			f := cmdutil.NewFakeFactory(out, tt.objs...)

			if tt.factoryOverrides != nil {
				tt.factoryOverrides(f)
			}

			// Default to plan command
			command := "plan"
			if tt.cmd != "" {
				// Override plan command
				command = tt.cmd
			}

			opts := &launcherOptions{command: command, runName: "run-12345"}

			// Mock the workspace controller by setting status up front
			var code int
			status := v1alpha1.RunStatus{
				Conditions: []metav1.Condition{
					{
						Type:   v1alpha1.RunCompleteCondition,
						Status: metav1.ConditionFalse,
						Reason: v1alpha1.PodRunningReason,
					},
				},
				Phase:    v1alpha1.RunPhaseRunning,
				ExitCode: &code,
			}
			// Permit individual tests to override workspace status
			if tt.overrideStatus != nil {
				tt.overrideStatus(&status)
			}
			opts.status = &status

			// create cobra command
			cmd := launcherCommand(f, opts)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			err = cmd.ExecuteContext(context.Background())
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Errorf("unexpected error: %w", err)
			}

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}
