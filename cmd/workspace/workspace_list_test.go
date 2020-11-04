package workspace

import (
	"bytes"
	"context"
	"testing"

	v1alpha1types "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestListWorkspaces(t *testing.T) {
	ws1 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}
	ws2 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-2",
			Namespace: "dev",
		},
	}

	tests := []struct {
		name string
		objs []runtime.Object
		args []string
		env  env.StokEnv
		err  bool
		out  string
	}{
		{
			name: "WithEnvironmentFile",
			objs: []runtime.Object{ws1, ws2},
			args: []string{},
			env:  env.StokEnv("default/workspace-1"),
			out:  "*\tdefault/workspace-1\n\tdev/workspace-2\n",
		},
		{
			name: "WithoutEnvironmentFile",
			objs: []runtime.Object{ws1, ws2},
			args: []string{},
			out:  "\tdefault/workspace-1\n\tdev/workspace-2\n",
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			path := t.NewTempDir().Chdir().Root()

			// Write .terraform/environment
			if tt.env != "" {
				require.NoError(t, tt.env.Write(path))
			}

			out := new(bytes.Buffer)

			opts, err := cmdutil.NewFakeOpts(out, tt.objs...)
			require.NoError(t, err)

			cmd := ListCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(opts.Out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Equal(t, tt.out, out.String())
		})
	}
}
