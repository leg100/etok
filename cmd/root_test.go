package cmd

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/version"
	"github.com/leg100/etok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoot(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		out        string
		err        bool
		setup      func()
		assertions func(*cmdutil.Options)
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
			name: "terraform command group",
			args: []string{"-h"},
			out:  "Terraform Commands:\n",
		},
		{
			name: "etok command group",
			args: []string{"-h"},
			out:  "etok Commands:\n",
		},
		{
			name: "version",
			args: []string{"version"},
			out:  "etok version 123\txyz\n",
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
		{
			name: "generate",
			args: []string{"generate"},
		},
		{
			name: "workspace",
			args: []string{"workspace"},
		},
		{
			name: "state",
			args: []string{"state"},
		},
		{
			name: "apply",
			args: []string{"apply", "-h"},
		},
		{
			name: "plan",
			args: []string{"plan", "-h"},
		},
		{
			name: "shell",
			args: []string{"sh", "-h"},
		},
		{
			name: "increased verbosity",
			args: []string{"-v=5"},
			// Cannot assert value of Verbosity because root's persistent run
			// hook is only executed for child commands (see below)
		},
		{
			// Check -v flag is persistent
			name: "increased verbosity on child command",
			args: []string{"version", "-v=5"},
			assertions: func(opts *cmdutil.Options) {
				assert.Equal(t, 5, opts.Verbosity)
			},
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

			cmd := RootCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Regexp(t, tt.out, out)

			if tt.assertions != nil {
				tt.assertions(opts)
			}
		})
	}
}
