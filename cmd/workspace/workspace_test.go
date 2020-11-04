package workspace

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			out:  "^Stok workspace management",
		},
		{
			name: "help",
			args: []string{"-h"},
			out:  "^Stok workspace management",
		},
		{
			name: "new",
			args: []string{"new", "-h"},
			out:  "^Create a new stok workspace",
		},
		{
			name: "list",
			args: []string{"list", "-h"},
			out:  "^List all workspaces",
		},
		{
			name: "delete",
			args: []string{"delete", "-h"},
			out:  "^Deletes a stok workspace",
		},
		{
			name: "show",
			args: []string{"show", "-h"},
			out:  "^Show current workspace",
		},
		{
			name: "select",
			args: []string{"select", "-h"},
			out:  "^Select a stok workspace",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.setup != nil {
				tt.setup()
			}
			out := new(bytes.Buffer)

			opts, err := cmdutil.NewFakeOpts(out)
			require.NoError(t, err)

			cmd := WorkspaceCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))
			assert.Regexp(t, tt.out, out)
		})
	}
}
