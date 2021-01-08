package workspace

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		out   string
		err   bool
		setup func()
	}{
		{
			name: "no args",
			args: []string{},
			out:  "^Etok workspace management",
		},
		{
			name: "help",
			args: []string{"-h"},
			out:  "^Etok workspace management",
		},
		{
			name: "new",
			args: []string{"new", "-h"},
			out:  "^Create a new etok workspace",
		},
		{
			name: "list",
			args: []string{"list", "-h"},
			out:  "^List all workspaces",
		},
		{
			name: "delete",
			args: []string{"delete", "-h"},
			out:  "^Deletes an etok workspace",
		},
		{
			name: "show",
			args: []string{"show", "-h"},
			out:  "^Show current workspace",
		},
		{
			name: "select",
			args: []string{"select", "-h"},
			out:  "^Select an etok workspace",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.setup != nil {
				tt.setup()
			}
			out := new(bytes.Buffer)

			f := cmdutil.NewFakeFactory(out)

			cmd := WorkspaceCmd(f)
			cmd.SetArgs(tt.args)
			cmd.SetOut(out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))
			assert.Regexp(t, tt.out, out)
		})
	}
}
