package workspace

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDeleteWorkspace(t *testing.T) {
	tests := []struct {
		name string
		args []string
		objs []runtime.Object
		err  bool
	}{
		{
			name: "With workspace",
			args: []string{"default/workspace-1"},
			objs: []runtime.Object{testobj.Workspace("default", "workspace-1")},
		},
		{
			name: "Without workspace",
			args: []string{"default/workspace-1"},
			err:  true,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			opts, err := cmdutil.NewFakeOpts(new(bytes.Buffer), tt.objs...)
			require.NoError(t, err)

			cmd := DeleteCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(opts.Out)
			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))
		})
	}
}
