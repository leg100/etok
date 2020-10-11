package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerraform(t *testing.T) {
	//workspaceObj := func(namespace, name string, queue ...string) *v1alpha1.Workspace {
	//	return &v1alpha1.Workspace{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name:      name,
	//			Namespace: namespace,
	//		},
	//		Status: v1alpha1.WorkspaceStatus{
	//			Conditions: status.Conditions{
	//				{
	//					Type:   v1alpha1.ConditionHealthy,
	//					Status: corev1.ConditionTrue,
	//				},
	//			},
	//			Queue: queue,
	//		},
	//	}
	//}

	//podReadyAndRunning := func(namespace, name string) *corev1.Pod {
	//	return &corev1.Pod{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name:      name,
	//			Namespace: namespace,
	//		},
	//		Status: corev1.PodStatus{
	//			Phase: corev1.PodRunning,
	//			Conditions: []corev1.PodCondition{
	//				{
	//					Type:   corev1.PodReady,
	//					Status: corev1.ConditionTrue,
	//				},
	//			},
	//		},
	//	}
	//}

	var cmdpaths [][]string
	for k, v := range run.TerraformCommandMap {
		if len(v) > 0 {
			for _, subcmd := range v {
				cmdpaths = append(cmdpaths, []string{k, subcmd})
			}
		} else {
			cmdpaths = append(cmdpaths, []string{k})
		}
	}

	for _, tfcmd := range cmdpaths {
		tests := []struct {
			name       string
			args       []string
			env        env.StokEnv
			err        bool
			assertions func(opts *app.Options)
		}{
			{
				name: strings.Join(tfcmd, "") + "WithDefaults",
				env:  env.StokEnv("default/default"),
				assertions: func(opts *app.Options) {
					assert.Equal(t, "default", opts.Namespace)
					assert.Equal(t, "default", opts.Workspace)
				},
			},
			{
				name: strings.Join(tfcmd, "") + "WithSpecificNamespaceAndWorkspace",
				env:  env.StokEnv("foo/bar"),
				assertions: func(opts *app.Options) {
					assert.Equal(t, "foo", opts.Namespace)
					assert.Equal(t, "bar", opts.Workspace)
				},
			},
			{
				name: strings.Join(tfcmd, "") + "WithSpecificNamespaceAndWorkspaceFlags",
				args: []string{"--debug", "--namespace", "foo", "--workspace", "bar"},
				env:  env.StokEnv("default/default"),
				assertions: func(opts *app.Options) {
					assert.Equal(t, "foo", opts.Namespace)
					assert.Equal(t, "bar", opts.Workspace)
				},
			},
			{
				name: strings.Join(tfcmd, "") + "WithTerraformFlag",
				args: []string{"--", "-input", "false"},
				env:  env.StokEnv("default/default"),
				assertions: func(opts *app.Options) {
					if tfcmd[0] == "sh" {
						assert.Equal(t, []string{"-c", "-input false"}, opts.Args)
					} else {
						assert.Equal(t, []string{"-input", "false"}, opts.Args)
					}
				},
			},
			{
				name: strings.Join(tfcmd, "") + "WithContextFlag",
				args: []string{"--debug", "--context", "oz-cluster"},
				env:  env.StokEnv("default/default"),
				assertions: func(opts *app.Options) {
					assert.Equal(t, "oz-cluster", opts.KubeContext)
				},
			},
			{
				name: strings.Join(tfcmd, "") + "WithoutStokEnv",
				assertions: func(opts *app.Options) {
					assert.Equal(t, "default", opts.Namespace)
					assert.Equal(t, "default", opts.Workspace)
				},
			},
		}

		for _, tt := range tests {
			testutil.Run(t, tt.name, func(t *testutil.T) {
				path := t.NewTempDir().Chdir().Root()

				// Write .terraform/environment
				if tt.env != "" {
					require.NoError(t, tt.env.Write(path))
				}

				opts, err := app.NewFakeOpts(new(bytes.Buffer))
				require.NoError(t, err)

				t.CheckError(tt.err, ParseArgs(context.Background(), append(tfcmd, tt.args...), opts))

				if tt.assertions != nil {
					tt.assertions(opts)
				}
			})
		}
	}
}
