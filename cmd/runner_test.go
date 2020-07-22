package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubectl/pkg/scheme"
)

func TestRunnerWithoutKind(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{"runner"})

	require.EqualError(t, err, "missing flag: --kind <kind>")
	require.Equal(t, 1, code)
}

func TestRunnerWithIncorrectKind(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "InvalidKind",
		"--tarball", "bad-tarball-zzz.tar.gz",
	})

	require.EqualError(t, err, "invalid kind: InvalidKind")
	require.Equal(t, 1, code)
}

func TestRunnerWithIncorrectTarball(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "Apply",
		"--tarball", "bad-tarball-zzz.tar.gz",
	})

	require.EqualError(t, err, "open bad-tarball-zzz.tar.gz: no such file or directory")
	require.Equal(t, 1, code)
}

func TestRunnerWithTarball(t *testing.T) {
	tarball := createTarballWithFiles(t, "test1.tf", "test2.tf")
	factory := fake.NewFactory(shell)
	dest := createTempPath(t)

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "Shell",
		"--name", "stok-shell-xyz",
		"--namespace", "test",
		"--tarball", tarball,
		"--path", dest,
		"--",
		"/bin/ls", filepath.Join(dest, "test1.tf"),
	})

	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestRunnerWithSpecificExitCode(t *testing.T) {
	tarball := createTarballWithFiles(t, "test1.tf", "test2.tf")
	factory := fake.NewFactory(shell)
	dest := createTempPath(t)

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "Shell",
		"--name", "stok-shell-xyz",
		"--namespace", "test",
		"--tarball", tarball,
		"--path", dest,
		"--",
		"exit", "101",
	})

	require.EqualError(t, err, "exit status 101")
	require.Equal(t, 101, code)
}

// Test interaction between launcher and client. Client sets annotation on command, runner waits for
// annotation to be unset, client unsets annotation, runner returns without error.
func TestRunnerWithAnnotationSetThenUnset(t *testing.T) {
	shell.SetAnnotations(map[string]string{v1alpha1.CommandWaitAnnotationKey: "true"})
	factory := fake.NewFactory(shell)

	// Get built-in scheme
	s := scheme.Scheme
	// And add our CRDs
	apis.AddToScheme(s)

	rc, err := factory.NewClient(s)
	require.NoError(t, err)

	done := make(chan error)
	go func() {
		done <- handleSemaphore(rc, s, "Shell", "stok-shell-xyz", "test", time.Second)
	}()

	// Wait for runner to poll twice before unsetting annotation
	wait.Poll(100*time.Millisecond, time.Second, func() (bool, error) {
		if factory.Gets > 1 {
			return true, nil
		}
		return false, nil
	})

	// Unset wait annotation
	shell.SetAnnotations(map[string]string{})
	require.NoError(t, factory.Client.Update(context.TODO(), shell))

	require.NoError(t, <-done)
}
