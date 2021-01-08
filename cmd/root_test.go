package cmd

import (
	"bytes"
	"context"
	"testing"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/version"
	"github.com/stretchr/testify/assert"
)

func TestRoot(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		out        string
		err        bool
		setup      func()
		assertions func(*cmdutil.Factory)
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
			name: "install",
			args: []string{"install", "-h"},
		},
		{
			name: "workspace",
			args: []string{"workspace"},
		},
		{
			name: "apply",
			args: []string{"apply", "-h"},
		},
		{
			name: "destroy",
			args: []string{"destroy", "-h"},
		},
		{
			name: "fmt",
			args: []string{"fmt", "-h"},
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
			assertions: func(f *cmdutil.Factory) {
				assert.Equal(t, 5, f.Verbosity)
			},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			if tt.setup != nil {
				tt.setup()
			}
			out := new(bytes.Buffer)

			f := cmdutil.NewFakeFactory(out)

			cmd := RootCmd(f)
			cmd.SetArgs(tt.args)
			cmd.SetOut(out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Regexp(t, tt.out, out)

			if tt.assertions != nil {
				tt.assertions(f)
			}
		})
	}
}
