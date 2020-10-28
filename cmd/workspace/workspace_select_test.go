package workspace

import (
	"bytes"
	"context"
	"testing"

	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceSelect(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  env.StokEnv
		out  string
		err  bool
	}{
		{
			name: "defaults",
			args: []string{"dev/networking"},
			env:  env.StokEnv("dev/networking"),
			out:  "Current workspace now: dev/networking\n",
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			path := t.NewTempDir().Chdir().Root()

			out := new(bytes.Buffer)

			opts, err := app.NewFakeOpts(out)
			require.NoError(t, err)

			cmd := SelectCmd(opts)
			cmd.SetArgs(tt.args)
			cmd.SetOut(opts.Out)

			t.CheckError(tt.err, cmd.ExecuteContext(context.Background()))

			assert.Equal(t, tt.out, out.String())

			// Confirm .terraform/environment was written with expected contents
			stokenv, err := env.ReadStokEnv(path)
			require.NoError(t, err)
			assert.Equal(t, tt.env, stokenv)
		})
	}
}
