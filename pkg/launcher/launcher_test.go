package launcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/attacher"
	etokclient "github.com/leg100/etok/pkg/client"
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
		err  error
		objs []runtime.Object
		// Override default command "plan"
		cmd string
		// Size of content to be archived
		size int
		// Mock exit code of runner container
		code int32
		// Override run status
		overrideStatus func(*v1alpha1.RunStatus)
		setOpts        func(*LauncherOptions)
		assertions     func(*testutil.T, *Launcher)
	}{
		{
			name: "plan",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			assertions: func(t *testutil.T, o *Launcher) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
		{
			name: "queueable commands",
			cmd:  "apply",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			assertions: func(t *testutil.T, o *Launcher) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
		{
			name: "specific namespace and workspace",
			objs: []runtime.Object{testobj.Workspace("foo", "bar", testobj.WithCombinedQueue("run-12345"))},
			setOpts: func(o *LauncherOptions) {
				o.Namespace = "foo"
				o.Workspace = "bar"
			},
			assertions: func(t *testutil.T, o *Launcher) {
				assert.Equal(t, "foo", o.Namespace)
				assert.Equal(t, "bar", o.Workspace)
			},
		},
		{
			name: "arbitrary terraform flag",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			setOpts: func(o *LauncherOptions) {
				o.Args = []string{"-input", "false"}
			},
			assertions: func(t *testutil.T, o *Launcher) {
				run, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, []string{"-input", "false"}, run.Args)
			},
		},
		{
			name: "approved",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"), testobj.WithPrivilegedCommands("plan"))},
			assertions: func(t *testutil.T, o *Launcher) {
				// Get run
				run, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				require.NoError(t, err)
				// Get workspace
				ws, err := o.WorkspacesClient(o.Namespace).Get(context.Background(), o.Workspace, metav1.GetOptions{})
				require.NoError(t, err)
				// Check run's approval annotation is set on workspace
				assert.Equal(t, true, ws.IsRunApproved(run))
			},
		},
		{
			name: "defaults",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			assertions: func(t *testutil.T, o *Launcher) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
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
			setOpts: func(o *LauncherOptions) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fakeError
				}
			},
			assertions: func(t *testutil.T, o *Launcher) {
				_, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))

				_, err = o.ConfigMapsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.True(t, kerrors.IsNotFound(err))
			},
		},
		{
			name: "disable cleanup resources upon error",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			err:  fakeError,
			setOpts: func(o *LauncherOptions) {
				o.DisableResourceCleanup = true

				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fakeError
				}
			},
			assertions: func(t *testutil.T, o *Launcher) {
				_, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ConfigMapsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "resources are not cleaned up upon exit code error",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			overrideStatus: func(status *v1alpha1.RunStatus) {
				// Mock pod returning exit code 5
				var code = 5
				status.ExitCode = &code
			},
			// Expect exit error with exit code 5
			err: etokerrors.NewExitError(5),
			assertions: func(t *testutil.T, o *Launcher) {
				_, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ConfigMapsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "with tty",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			setOpts: func(o *LauncherOptions) {
				var err error
				o.In, _, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(t *testutil.T, o *Launcher) {
				// With a tty, launcher should attach not stream logs
				assert.Equal(t, "fake attach", o.Out.(*bytes.Buffer).String())

				// Get run
				run, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				require.NoError(t, err)
				// With a tty, a handshake is required
				assert.True(t, run.Handshake)
			},
		},
		{
			name: "disable tty",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			setOpts: func(o *LauncherOptions) {
				o.DisableTTY = true

				// Ensure tty is overridden
				var err error
				_, o.In, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(t *testutil.T, o *Launcher) {
				// With tty disabled, launcher should stream logs not attach
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())

				// Get run
				run, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
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
			setOpts: func(o *LauncherOptions) {
				var err error
				_, o.In, err = pty.Open()
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
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			setOpts: func(o *LauncherOptions) {
				o.ReconcileTimeout = 10 * time.Millisecond
			},
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
			err: handlers.ErrRunFailed,
		},
		{
			name: "increased verbosity",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"), testobj.WithPrivilegedCommands("plan"))},
			setOpts: func(o *LauncherOptions) {
				o.Verbosity = 3
			},
			assertions: func(t *testutil.T, l *Launcher) {
				// Get run
				run, err := l.RunsClient(l.Namespace).Get(context.Background(), l.RunName, metav1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, 3, run.Verbosity)
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.NewTempDir().Chdir().WriteRandomFile("test.bin", tt.size)

			out := new(bytes.Buffer)

			// Default to plan command
			command := "plan"
			if tt.cmd != "" {
				// Override plan command
				command = tt.cmd
			}

			// Create k8s clients
			cc := etokclient.NewFakeClientCreator(tt.objs...)
			client, err := cc.Create("")
			require.NoError(t, err)

			opts := &LauncherOptions{
				AttachFunc:  attacher.FakeAttach,
				Client:      client,
				Command:     command,
				GetLogsFunc: logstreamer.FakeGetLogs,
				RunName:     "run-12345",
				IOStreams: &cmdutil.IOStreams{
					Out: out,
				},
			}

			// Permit individual tests to override options
			if tt.setOpts != nil {
				tt.setOpts(opts)
			}

			l := NewLauncher(opts)

			// Mock the run controller by setting status up front
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
			// Permit individual tests to override run status
			if tt.overrideStatus != nil {
				tt.overrideStatus(&status)
			}
			l.Status = &status

			err = l.Launch(context.Background())
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Errorf("unexpected error: %w", err)
			}

			if tt.assertions != nil {
				tt.assertions(t, l)
			}
		})
	}
}
