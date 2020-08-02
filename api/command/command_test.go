package command

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunnerArgsForKindShellNoArgs(t *testing.T) {
	got := RunnerArgsForKind("Shell", []string{})
	want := []string{"/bin/sh"}
	require.Equal(t, want, got)
}

func TestRunnerArgsForKindShellWithArgs(t *testing.T) {
	got := RunnerArgsForKind("Shell", []string{"/bin/echo", "foo"})
	want := []string{"/bin/sh", "-c", "/bin/echo foo"}
	require.Equal(t, want, got)
}

func TestRunnerArgsForKindWorkspaceWithArgs(t *testing.T) {
	got := RunnerArgsForKind("Workspace", []string{"-backend-config=backend.ini"})
	want := []string{"terraform", "init", "-backend-config=backend.ini"}
	require.Equal(t, want, got)
}

func TestRunnerArgsForKindPlan(t *testing.T) {
	got := RunnerArgsForKind("Plan", []string{})
	want := []string{"terraform", "plan"}
	require.Equal(t, want, got)
}
