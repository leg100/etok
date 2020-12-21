package workspace

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/env"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestListWorkspaces(t *testing.T) {
	tests := []struct {
		name string
		objs []runtime.Object
		args []string
		env  *env.Env
		err  bool
		out  string
	}{
		{
			name: "WithEnvironmentFile",
			objs: []runtime.Object{
				testobj.Workspace("default", "workspace-1"),
				testobj.Workspace("dev", "workspace-2"),
			},
			args: []string{},
			env:  &env.Env{Namespace: "default", Workspace: "workspace-1"},
			out:  "*\tdefault_workspace-1\n\tdev_workspace-2\n",
		},
		{
			name: "WithoutEnvironmentFile",
			objs: []runtime.Object{
				testobj.Workspace("default", "workspace-1"),
				testobj.Workspace("dev", "workspace-2"),
			},
			args: []string{},
			out:  "\tdefault_workspace-1\n\tdev_workspace-2\n",
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

			opts, err := cmdutil.NewFakeOpts(out, tt.objs...)
			require.NoError(t, err)

			cmd := listCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(opts.Out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Equal(t, tt.out, out.String())
		})
	}
}
