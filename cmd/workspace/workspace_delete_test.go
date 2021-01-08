package workspace

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
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
			args: []string{"workspace-1"},
			objs: []runtime.Object{testobj.Workspace("default", "workspace-1")},
		},
		{
			name: "Without workspace",
			args: []string{"workspace-1"},
			err:  true,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			f := cmdutil.NewFakeFactory(new(bytes.Buffer), tt.objs...)

			cmd := deleteCmd(f)
			cmd.SetArgs(tt.args)
			cmd.SetOut(f.Out)
			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))
		})
	}
}
