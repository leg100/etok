package launcher

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
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
		assertions func(o *LauncherOptions)
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
	}

	for _, tt := range tests {
		var leafCmds []string
		for k, v := range run.TerraformCommandMap {
			if len(v) > 0 {
				for _, subcmd := range v {
					leafCmds = append(leafCmds, subcmd)
				}
			} else {
				leafCmds = append(leafCmds, k)
			}
		}

		for _, lcmd := range leafCmds {
			name := lcmd + tt.name
			testutil.Run(t, name, func(t *testutil.T) {
				path := t.NewTempDir().Chdir().Root()

				// Write .terraform/environment
				if tt.env != "" {
					require.NoError(t, tt.env.Write(path))
				}

				out := new(bytes.Buffer)
				opts, err := app.NewFakeOpts(out, tt.objs...)
				require.NoError(t, err)

				cmd, cmdOpts := LauncherCmd(opts, lcmd)
				cmd.SetOut(out)
				cmd.SetArgs(tt.args)

				mockRunController(opts, cmdOpts)

				// Set debug flag (that root cmd otherwise sets)
				cmd.Flags().BoolVar(&cmdOpts.Debug, "debug", true, "debug flag")

				t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

				if tt.assertions != nil {
					tt.assertions(cmdOpts)
				}
			})
		}
	}
}

// When a run create event occurs create a pod
func mockRunController(opts *app.Options, o *LauncherOptions) {
	createPodAction := func(action testcore.Action) (bool, runtime.Object, error) {
		run := action.(testcore.CreateAction).GetObject().(*v1alpha1.Run)
		pod := testPod(run.GetNamespace(), run.GetName())
		o.PodsClient(run.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})

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
