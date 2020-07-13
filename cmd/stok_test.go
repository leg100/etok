package cmd

import (
	"bytes"
	"testing"

	"github.com/leg100/stok/version"
	"github.com/stretchr/testify/require"
)

type exitMock struct {
	code int
}

func (e *exitMock) Exit(code int) {
	e.code = code
}

func TestStokNoArgs(t *testing.T) {
	var e = &exitMock{}
	Execute([]string{""}, e.Exit)

	require.Equal(t, 0, e.code)
}

func TestStokHelp(t *testing.T) {
	var out bytes.Buffer
	var e = &exitMock{}
	var cmd = newStokCmd(e.Exit).cmd

	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-h"})

	require.NoError(t, cmd.Execute())
	require.Regexp(t, "^Supercharge terraform on kubernetes\n", out.String())
	require.Equal(t, 0, e.code)
}

func TestStokVersion(t *testing.T) {
	version.Version = "123"
	version.Commit = "xyz"

	var out bytes.Buffer
	var e = &exitMock{}
	var cmd = newStokCmd(e.Exit).cmd

	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-v"})

	require.NoError(t, cmd.Execute())
	require.Equal(t, "stok version 123\txyz\n", out.String())
	require.Equal(t, 0, e.code)
}

func TestStokDebug(t *testing.T) {
	var out bytes.Buffer
	var e = &exitMock{}
	var cmd = newStokCmd(e.Exit).cmd

	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--debug"})

	require.NoError(t, cmd.Execute())
	require.Equal(t, 0, e.code)
}
