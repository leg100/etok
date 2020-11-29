package main

import (
	"bytes"
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/leg100/stok/version"
	"github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		out   string
		err   bool
		setup func()
		code  int
	}{
		{
			name: "no args",
			args: []string{},
			out:  "^Usage:",
		},
		{
			name: "help",
			args: []string{"-h"},
			out:  "^Usage:",
		},
		{
			name: "version",
			args: []string{"version"},
			out:  "stok version 123\txyz\n",
			setup: func() {
				version.Version = "123"
				version.Commit = "xyz"
			},
		},
		{
			name: "increased verbosity",
			args: []string{"-v=5"},
		},
		{
			name: "invalid",
			args: []string{"invalid"},
			err:  true,
			code: 1,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.setup != nil {
				tt.setup()
			}
			out := new(bytes.Buffer)

			err := run(tt.args, out, new(bytes.Buffer), new(bytes.Buffer))

			assert.Regexp(t, tt.out, out)

			if tt.err {
				// Check exit code and check stderr is not empty
				errOut := new(bytes.Buffer)
				assert.Equal(t, tt.code, handleError(err, errOut))
				assert.NotEmpty(t, errOut)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
