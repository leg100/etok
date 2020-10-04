package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/leg100/stok/version"
	"github.com/stretchr/testify/assert"
)

func TestRoot(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		out   string
		code  int
		setup func()
	}{
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
			name: "no args",
			args: []string{},
			code: 1,
		},
		{
			name: "help",
			args: []string{"-h"},
			out:  "^Usage\n",
			code: 1,
		},
		{
			name: "invalid",
			args: []string{"invalid"},
			code: 1,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.setup != nil {
				tt.setup()
			}
			out := new(bytes.Buffer)

			code, _ := ExecWithExitCode(context.Background(), tt.args, out, out)
			//fmt.Printf("out: %v\n", out.String())

			//require.NoError(t, err)
			assert.Equal(t, tt.code, code)
			assert.Regexp(t, tt.out, out)
		})
	}
}
