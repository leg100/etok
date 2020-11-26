package launcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/creack/pty"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/logstreamer"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testcore "k8s.io/client-go/testing"
)

func TestLauncher(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      env.StokEnv
		err      bool
		objs     []runtime.Object
		podPhase corev1.PodPhase
		// Size of content to be archived
		size int
		// Mock exit code of runner pod
		code       int32
		setOpts    func(*cmdutil.Options)
		assertions func(*LauncherOptions)
	}{
		{
			name: "defaults",
			env:  env.StokEnv("default/default"),
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
		{
			name: "specific namespace and workspace",
			env:  env.StokEnv("foo/bar"),
			objs: []runtime.Object{testobj.Workspace("foo", "bar")},
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "foo", o.Namespace)
				assert.Equal(t, "bar", o.Workspace)
			},
		},
		{
			name: "workspace flag",
			args: []string{"--workspace", "foo/bar"},
			objs: []runtime.Object{testobj.Workspace("foo", "bar")},
			env:  env.StokEnv("default/default"),
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "foo", o.Namespace)
				assert.Equal(t, "bar", o.Workspace)
			},
		},
		{
			name: "arbitrary terraform flag",
			args: []string{"--", "-input", "false"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			env:  env.StokEnv("default/default"),
			assertions: func(o *LauncherOptions) {
				if o.Command == "sh" {
					assert.Equal(t, []string{"-c", "-input false"}, o.args)
				} else {
					assert.Equal(t, []string{"-input", "false"}, o.args)
				}
			},
		},
		{
			name: "context flag",
			args: []string{"--context", "oz-cluster"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			env:  env.StokEnv("default/default"),
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "oz-cluster", o.KubeContext)
			},
		},
		{
			name: "debug",
			args: []string{"--debug"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			assertions: func(o *LauncherOptions) {
				run, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, true, run.GetDebug())
			},
		},
		{
			name: "approved",
			args: []string{"--debug"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithPrivilegedCommands(allCommands...))},
			assertions: func(o *LauncherOptions) {
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
			name: "without env file",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
		{
			name: "workspace does not exist",
			err:  true,
		},
		{
			name: "cleanup resources upon error",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			err:  true,
			setOpts: func(o *cmdutil.Options) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fmt.Errorf("fake error")
				}
			},
			assertions: func(o *LauncherOptions) {
				_, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))

				_, err = o.ConfigMapsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.True(t, errors.IsNotFound(err))
			},
		},
		{
			name: "disable cleanup resources upon error",
			args: []string{"--no-cleanup"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			err:  true,
			setOpts: func(o *cmdutil.Options) {
				o.GetLogsFunc = func(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
					return nil, fmt.Errorf("fake error")
				}
			},
			assertions: func(o *LauncherOptions) {
				_, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ConfigMapsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "resources are not cleaned up upon exit code error",
			args: []string{},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			err:  true,
			code: int32(5),
			assertions: func(o *LauncherOptions) {
				_, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)

				_, err = o.ConfigMapsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				assert.NoError(t, err)
			},
		},
		{
			name: "with tty",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			setOpts: func(opts *cmdutil.Options) {
				var err error
				opts.In, _, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(o *LauncherOptions) {
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
			args: []string{"--no-tty"},
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			setOpts: func(opts *cmdutil.Options) {
				// Ensure tty is overridden
				var err error
				_, opts.In, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(o *LauncherOptions) {
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
			name:     "pod completed with no tty",
			objs:     []runtime.Object{testobj.Workspace("default", "default")},
			podPhase: corev1.PodSucceeded,
		},
		{
			name: "pod completed with tty",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			setOpts: func(opts *cmdutil.Options) {
				var err error
				_, opts.In, err = pty.Open()
				require.NoError(t, err)
			},
			podPhase: corev1.PodSucceeded,
			err:      true,
		},
		{
			name: "config too big",
			objs: []runtime.Object{testobj.Workspace("default", "default")},
			size: 1024*1024 + 1,
			err:  true,
		},
	}

	for _, tt := range tests {
		cmdFactories := nonStateCommands()
		cmdFactories = append(cmdFactories, stateSubCommands()...)
		cmdFactories = append(cmdFactories, shellCommand())

		for _, f := range cmdFactories {
			testutil.Run(t, tt.name+"/"+f.name, func(t *testutil.T) {
				path := t.NewTempDir().Chdir().WriteRandomFile("test.bin", tt.size).Root()

				// Write .terraform/environment
				if tt.env != "" {
					require.NoError(t, tt.env.Write(path))
				}

				out := new(bytes.Buffer)
				opts, err := cmdutil.NewFakeOpts(out, tt.objs...)
				require.NoError(t, err)

				if tt.setOpts != nil {
					tt.setOpts(opts)
				}

				cmdOpts := &LauncherOptions{}

				// create cobra command
				cmd := f.create(opts, cmdOpts)
				cmd.SetOut(out)
				cmd.SetArgs(tt.args)

				mockControllers(t, opts, cmdOpts, tt.podPhase, tt.code)

				// Set debug flag (that root cmd otherwise sets)
				cmd.Flags().BoolVar(&cmdOpts.Debug, "debug", false, "debug flag")

				t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

				if tt.assertions != nil {
					tt.assertions(cmdOpts)
				}
			})
		}
	}
}

// Mock controllers (badly):
// (a) Runs controller: When a run is created, create a pod
// (b) Pods controller: Simulate pod completing successfully
func mockControllers(t *testutil.T, opts *cmdutil.Options, o *LauncherOptions, phase corev1.PodPhase, exitCode int32) {
	createPodAction := func(action testcore.Action) (bool, runtime.Object, error) {
		run := action.(testcore.CreateAction).GetObject().(*v1alpha1.Run)

		pod := testobj.RunPod(run.Namespace, run.Name, testobj.WithPhase(phase), testobj.WithExitCode(exitCode))
		_, err := o.PodsClient(run.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})
		require.NoError(t, err)

		return false, action.(testcore.CreateAction).GetObject(), nil
	}

	opts.ClientCreator.(*client.FakeClientCreator).PrependReactor("create", "runs", createPodAction)
}
