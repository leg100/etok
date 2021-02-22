package runs

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/launcher"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestLauncher(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  *env.Env
		err  error
		objs []runtime.Object
		// Override default command "plan"
		cmd              string
		factoryOverrides func(*cmdutil.Factory)
		assertions       func(*launcher.LauncherOptions)
	}{
		{
			name: "namespace flag overrides environment",
			args: []string{"--namespace", "foo"},
			objs: []runtime.Object{testobj.Workspace("foo", "default", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcher.LauncherOptions) {
				assert.Equal(t, "foo", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
		{
			name: "workspace flag overrides environment",
			args: []string{"--workspace", "bar"},
			objs: []runtime.Object{testobj.Workspace("default", "bar", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcher.LauncherOptions) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "bar", o.Workspace)
			},
		},
		{
			name: "arbitrary terraform flag",
			args: []string{"--", "-input", "false"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcher.LauncherOptions) {
				assert.Equal(t, []string{"-input", "false"}, o.Args)
			},
		},
		{
			name: "context flag",
			args: []string{"--context", "oz-cluster"},
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			env:  &env.Env{Namespace: "default", Workspace: "default"},
			assertions: func(o *launcher.LauncherOptions) {
				assert.Equal(t, "oz-cluster", o.KubeContext)
			},
		},
		{
			name: "approved",
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"), testobj.WithPrivilegedCommands("plan"))},
			assertions: func(o *launcher.LauncherOptions) {
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
			objs: []runtime.Object{testobj.Workspace("default", "default", testobj.WithCombinedQueue("run-12345"))},
			assertions: func(o *launcher.LauncherOptions) {
				assert.Equal(t, "default", o.Namespace)
				assert.Equal(t, "default", o.Workspace)
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			path := t.NewTempDir().Chdir().Root()

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

			opts := &launcher.LauncherOptions{Command: command, RunName: "run-12345"}

			// Mock the run controller by setting status up front
			var code int
			opts.Status = &v1alpha1.RunStatus{
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

			// create cobra command
			cmd := runCommand(f, opts)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			err := cmd.ExecuteContext(context.Background())
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Errorf("unexpected error: %w", err)
			}

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}
