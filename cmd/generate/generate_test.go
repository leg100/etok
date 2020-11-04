package generate

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
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
			out:  "^Generate deployment resources",
		},
		{
			name: "help",
			args: []string{"-h"},
			out:  "^Generate deployment resources",
		},
		{
			name: "crds",
			args: []string{"crds", "-h"},
			out:  "Generate stok CRDs",
		},
		{
			name: "operator",
			args: []string{"operator", "-h"},
			out:  "Generate operator's kubernetes resources",
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

			cmd := GenerateCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))
			assert.Regexp(t, tt.out, out)
		})
	}
}
