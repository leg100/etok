package cmd

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
			opts, err := app.NewFakeOpts(new(bytes.Buffer))
			require.NoError(t, err)

			t.CheckError(tt.err, ParseArgs(context.Background(), tt.args, opts))
		})
	}
}
