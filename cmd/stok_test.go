package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/leg100/stok/version"
	"github.com/stretchr/testify/require"
)

func TestStokNoArgs(t *testing.T) {
	out := new(bytes.Buffer)
	cmd := newStokCmd(out, out)
	cmd.cmd.SetOut(out)

	code, _ := cmd.Execute([]string{"-v"})

	require.Equal(t, 0, code)
}

func TestStokHelp(t *testing.T) {
	var out bytes.Buffer
	var cmd = newStokCmd(os.Stdout, os.Stderr)

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
	var cmd = newStokCmd(os.Stdout, os.Stderr)

	cmd.cmd.SetOut(&out)
	code, err := cmd.Execute([]string{"-v"})

	require.NoError(t, err)
	require.Equal(t, "stok version 123\txyz\n", out.String())
	require.Equal(t, 0, code)
}

func TestStokDebug(t *testing.T) {
	out := new(bytes.Buffer)

	cmd := newStokCmd(out, out)
	cmd.cmd.SetOut(out)
	code, err := cmd.Execute([]string{"--debug"})

	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestStokInvalidCommand(t *testing.T) {
	out := new(bytes.Buffer)

	code, err := newStokCmd(out, out).Execute([]string{"invalid"})

	require.Error(t, err)
	require.Equal(t, 1, code)
}
