package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRunnerWithoutKind(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{"runner"})

	require.EqualError(t, err, "runner: missing flag: --kind <kind>")
	require.Equal(t, 1, code)
}

func TestRunnerWithIncorrectKind(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "InvalidKind",
		"--tarball", "bad-tarball-zzz.tar.gz",
	})

	require.EqualError(t, err, "runner: invalid kind: InvalidKind")
	require.Equal(t, 1, code)
}

func TestRunnerWithIncorrectTarball(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "Apply",
		"--tarball", "bad-tarball-zzz.tar.gz",
	})

	require.EqualError(t, err, "runner: open bad-tarball-zzz.tar.gz: no such file or directory")
	require.Equal(t, 1, code)
}

func TestRunnerWithWaitDisabled(t *testing.T) {
	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "Shell",
		"--no-wait",
		"--",
		"uname",
	})

	require.NoError(t, err)
	require.Equal(t, 0, code)
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

func TestRunnerWithoutTarball(t *testing.T) {
	factory := fake.NewFactory(shell)

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"runner",
		"--kind", "Shell",
		"--name", "stok-shell-xyz",
		"--namespace", "test",
		"--",
		"date",
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

	require.EqualError(t, err, "runner: exit status 101")
	require.Equal(t, 101, code)
}

// Test interaction between launcher and client. Client sets annotation on command, runner waits for
// annotation to be unset, client unsets annotation, runner returns without error.
func TestRunnerWithAnnotationSetThenUnset(t *testing.T) {
	shell.SetAnnotations(map[string]string{v1alpha1.WaitAnnotationKey: "true"})
	factory := fake.NewFactory(shell)

	rc, err := factory.NewClient(scheme.Scheme, "")
	require.NoError(t, err)

	gets := make(chan struct{})

	// Everytime a get request is made, send a msg to increment the tally
	factory.AddReactor("get", func(cr runtimeclient.Client, ctx context.Context, key runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
		gets <- struct{}{}
		return obj, nil
	})

	done := make(chan error)
	go func() {
		done <- handleSemaphore(rc, scheme.Scheme, "Shell", "stok-shell-xyz", "test", 5*time.Second, 100*time.Millisecond)
	}()

	// Running tally of get requests
	var tally int
	select {
	case <-gets:
		tally++
		// if at least two gets have been made, then release hold
		if tally > 1 {
			// Unset wait annotation
			shell.SetAnnotations(map[string]string{})
			factory.Client.Update(context.TODO(), shell)
		}
	case err := <-done:
		require.NoError(t, err)
		return
	case <-time.After(3 * time.Second):
		t.Errorf("timed out waiting for test to complete")
		return
	}
}
