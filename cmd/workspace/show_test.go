package workspace

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceShow(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  env.StokEnv
		out  string
		code int
	}{
		{
			name: "WithEnvironmentFile",
			args: []string{"workspace", "show"},
			env:  env.StokEnv("default/workspace-1"),
			out:  "default/workspace-1",
		},
		{
			name: "WithoutEnvironmentFile",
			args: []string{"workspace", "show"},
			out:  "no such file or directory",
			code: 1,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			path := t.NewTempDir().Chdir().Root()

			// Write .terraform/environment
			if tt.env != "" {
				require.NoError(t, tt.env.Write(path))
			}

			out := new(bytes.Buffer)
			code, _ := newStokCmd(tt.args, out, out).Execute()

			require.Equal(t, tt.code, code)

			// Merely ensure expected output is a subset of actual output (no such file error
			// messages include the temporary directory name which isn't known to the test case in
			// the table above)
			require.Regexp(t, regexp.MustCompile(tt.out), out.String())
		})
	}
}
