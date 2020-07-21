package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/leg100/stok/version"
	"github.com/stretchr/testify/require"
)

func TestStokNoArgs(t *testing.T) {
	require.Equal(t, 0, Execute([]string{""}))
}

func TestStokHelp(t *testing.T) {
	var out bytes.Buffer
	var cmd = newStokCmd(&fake.Factory{}, os.Stdout, os.Stderr)

	cmd.cmd.SetOut(&out)
	code, err := cmd.Execute([]string{"-h"})

	require.NoError(t, err)
	require.Regexp(t, "^Supercharge terraform on kubernetes\n", out.String())
	require.Equal(t, 0, code)
}

func TestStokVersion(t *testing.T) {
	version.Version = "123"
	version.Commit = "xyz"

	var out bytes.Buffer
	var cmd = newStokCmd(&fake.Factory{}, os.Stdout, os.Stderr)

	cmd.cmd.SetOut(&out)
	code, err := cmd.Execute([]string{"-v"})

	require.NoError(t, err)
	require.Equal(t, "stok version 123\txyz\n", out.String())
	require.Equal(t, 0, code)
}

func TestStokDebug(t *testing.T) {
	var cmd = newStokCmd(&fake.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{"--debug"})

	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestStokInvalidCommand(t *testing.T) {
	var cmd = newStokCmd(&fake.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{"invalid"})

	require.Error(t, err)
	require.Equal(t, 1, code)
}
