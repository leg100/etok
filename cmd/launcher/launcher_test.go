package launcher

import (
	"bytes"
	"context"
	"testing"

	"github.com/kr/pty"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/client"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testcore "k8s.io/client-go/testing"
)

func TestLauncher(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		env        env.StokEnv
		err        bool
		objs       []runtime.Object
		setOpts    func(*cmdutil.Options)
		assertions func(*LauncherOptions)
	}{
		{
			name: "defaults",
			env:  env.StokEnv("default/default"),
			objs: []runtime.Object{testWorkspace("default", "default")},
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
		{
			name: "specific namespace and workspace",
			env:  env.StokEnv("foo/bar"),
			objs: []runtime.Object{testWorkspace("foo", "bar")},
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "foo", o.Namespace)
				assert.Equal(t, "bar", o.Workspace)
			},
		},
		{
			name: "specific namespace and workspace flags",
			args: []string{"--namespace", "foo", "--workspace", "bar"},
			objs: []runtime.Object{testWorkspace("foo", "bar")},
			env:  env.StokEnv("default/default"),
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "foo", o.Namespace)
				assert.Equal(t, "bar", o.Workspace)
			},
		},
		{
			name: "arbitrary terraform flag",
			args: []string{"--", "-input", "false"},
			objs: []runtime.Object{testWorkspace("default", "default")},
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
			objs: []runtime.Object{testWorkspace("default", "default")},
			env:  env.StokEnv("default/default"),
			assertions: func(o *LauncherOptions) {
				assert.Equal(t, "oz-cluster", o.KubeContext)
			},
		},
		{
			name: "debug",
			args: []string{"--debug"},
			objs: []runtime.Object{testWorkspace("default", "default")},
			assertions: func(o *LauncherOptions) {
				run, err := o.RunsClient(o.Namespace).Get(context.Background(), o.RunName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, true, run.GetDebug())
			},
		},
		{
			name: "without env file",
			objs: []runtime.Object{testWorkspace("default", "default")},
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
			name: "with tty",
			objs: []runtime.Object{testWorkspace("default", "default")},
			setOpts: func(opts *cmdutil.Options) {
				var err error
				opts.In, _, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(o *LauncherOptions) {
				// With a tty, launcher should attach not stream logs
				assert.Equal(t, "fake attach", o.Out.(*bytes.Buffer).String())
			},
		},
		{
			name: "disable tty",
			args: []string{"--no-tty"},
			objs: []runtime.Object{testWorkspace("default", "default")},
			setOpts: func(opts *cmdutil.Options) {
				// Ensure tty is overridden
				var err error
				_, opts.In, err = pty.Open()
				require.NoError(t, err)
			},
			assertions: func(o *LauncherOptions) {
				// With tty disabled, launcher should stream logs not attach
				assert.Equal(t, "fake logs", o.Out.(*bytes.Buffer).String())
			},
		},
	}

	for _, tt := range tests {
		cmdFactories := nonStateCommands()
		cmdFactories = append(cmdFactories, stateSubCommands()...)
		cmdFactories = append(cmdFactories, shellCommand())

		for _, f := range cmdFactories {
			testutil.Run(t, tt.name+"/"+f.name, func(t *testutil.T) {
				path := t.NewTempDir().Chdir().Root()

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

				mockControllers(opts, cmdOpts)

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
func mockControllers(opts *cmdutil.Options, o *LauncherOptions) {
	createPodAction := func(action testcore.Action) (bool, runtime.Object, error) {
		run := action.(testcore.CreateAction).GetObject().(*v1alpha1.Run)

		pod := testPod(run.GetNamespace(), run.GetName())
		_, _ = o.PodsClient(run.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})

		return false, action.(testcore.CreateAction).GetObject(), nil
	}

	opts.ClientCreator.(*client.FakeClientCreator).PrependReactor("create", "runs", createPodAction)
}

func testPod(namespace, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			},
		},
	}
}

func testWorkspace(namespace, name string) *v1alpha1.Workspace {
	return &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
