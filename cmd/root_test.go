package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/testutil"
	"github.com/leg100/stok/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoot(t *testing.T) {
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
			out:  "^Usage: stok \\[command\\]",
		},
		{
			name: "help",
			args: []string{"-h"},
			out:  "^Usage: stok \\[command\\]",
		},
		{
			name: "version",
			args: []string{"-v"},
			out:  "stok version 123\txyz\n",
			setup: func() {
				version.Version = "123"
				version.Commit = "xyz"
			},
		},
		{
			name: "invalid",
			args: []string{"invalid"},
			err:  true,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.setup != nil {
				tt.setup()
			}
			out := new(bytes.Buffer)

			opts, err := app.NewFakeOpts(out)
			require.NoError(t, err)

			t.CheckError(tt.err, ParseArgs(context.Background(), tt.args, opts))
			assert.Regexp(t, tt.out, out)
		})
	}
}
