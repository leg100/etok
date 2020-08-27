package cmd

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/leg100/stok/util"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceShow(t *testing.T) {
	t.Run("WithEnvironmentFile", func(t *testing.T) {
		path := createTempPath(t)
		err := util.WriteEnvironmentFile(path, "default", "workspace-1")
		require.NoError(t, err)

		var out = new(bytes.Buffer)
		var cmd = newStokCmd(&fake.Factory{}, out, out)

		code, err := cmd.Execute([]string{
			"workspace",
			"show",
			"--path", path,
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
		require.Equal(t, "default/workspace-1\n", out.String())
	})

	t.Run("WithoutEnvironmentFile", func(t *testing.T) {
		path := createTempPath(t)

		var out = new(bytes.Buffer)
		var cmd = newStokCmd(&fake.Factory{}, out, out)

		code, err := cmd.Execute([]string{
			"workspace",
			"show",
			"--path", path,
		})
		require.Error(t, err)
		require.Equal(t, 1, code)
		require.Regexp(t, regexp.MustCompile("no such file or directory"), out.String())
	})
}
