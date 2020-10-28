package generate

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
)

func TestGenerateOperator(t *testing.T) {
	tests := []struct {
		name string
		args []string
		err  bool
	}{
		{
			name: "defaults",
			args: []string{"generate", "operator"},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			opts, err := app.NewFakeOpts(out)
			require.NoError(t, err)

			cmd, _ := GenerateOperatorCmd(opts)
			cmd.SetOut(out)
			cmd.SetArgs(tt.args)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))
		})
	}
}
