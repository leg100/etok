package cmd

import (
	"bytes"
	"testing"

	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
)

func TestRunner(t *testing.T) {
	//shellWithoutAnnotation := &v1alpha1.Run{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      "run-xyz",
	//		Namespace: "test",
	//	},
	//	RunSpec: v1alpha1.RunSpec{
	//		Command: "sh",
	//		Args:    []string{"cow", "pig"},
	//	},
	//}

	tests := []struct {
		args []string
		err  string
		code int
	}{
		{
			args: []string{"runner"},
			err:  "runner: invalid kind: ",
			code: 1,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, "WithoutKind", func(t *testutil.T) {
			out := new(bytes.Buffer)
			code, err := newStokCmd(out, out).Execute(tt.args)

			require.EqualError(t, err, tt.err)
			require.Equal(t, 1, code)
		})
	}

	//t.Run("WithIncorrectKind", func(t *testing.T) {
	//	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "InvalidKind",
	//		"--tarball", "bad-tarball-zzz.tar.gz",
	//	})

	//	require.EqualError(t, err, "runner: invalid kind: InvalidKind")
	//	require.Equal(t, 1, code)
	//})

	//t.Run("WithIncorrectTarball", func(t *testing.T) {
	//	factory := fake.NewFactory(shellWithoutAnnotation)
	//	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--tarball", "bad-tarball-zzz.tar.gz",
	//	})

	//	require.EqualError(t, err, "runner: open bad-tarball-zzz.tar.gz: no such file or directory")
	//	require.Equal(t, 1, code)
	//})

	//t.Run("WithWaitDisabled", func(t *testing.T) {
	//	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--no-wait",
	//		"--",
	//		"uname",
	//	})

	//	require.NoError(t, err)
	//	require.Equal(t, 0, code)
	//})

	//t.Run("WithEnvVar", func(t *testing.T) {
	//	(&testutil.T{T: t}).SetEnvs(map[string]string{"STOK_KIND": "Run"})

	//	var cmd = newStokCmd(&k8s.Factory{}, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--no-wait",
	//		"--",
	//		"uname",
	//	})

	//	require.NoError(t, err)
	//	require.Equal(t, 0, code)
	//})

	//t.Run("WithTarball", func(t *testing.T) {
	//	tarball := createTarballWithFiles(t, "test1.tf", "test2.tf")
	//	factory := fake.NewFactory(shellWithoutAnnotation)
	//	dest := createTempPath(t)

	//	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--name", "run-xyz",
	//		"--namespace", "test",
	//		"--tarball", tarball,
	//		"--path", dest,
	//		"--no-wait",
	//		"--",
	//		"/bin/ls", filepath.Join(dest, "test1.tf"),
	//	})

	//	require.NoError(t, err)
	//	require.Equal(t, 0, code)
	//})

	//t.Run("WithTarballArgOverridingEnvVar", func(t *testing.T) {
	//	(&testutil.T{T: t}).SetEnvs(map[string]string{"STOK_TARBALL": "doesnotexist.tar.gz"})

	//	tarball := createTarballWithFiles(t, "test1.tf", "test2.tf")
	//	factory := fake.NewFactory(shellWithoutAnnotation)
	//	dest := createTempPath(t)

	//	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--name", "run-xyz",
	//		"--namespace", "test",
	//		"--tarball", tarball,
	//		"--path", dest,
	//		"--no-wait",
	//		"--",
	//		"/bin/ls", filepath.Join(dest, "test1.tf"),
	//	})

	//	require.NoError(t, err)
	//	require.Equal(t, 0, code)
	//})

	//t.Run("WithoutTarball", func(t *testing.T) {
	//	factory := fake.NewFactory(shellWithoutAnnotation)

	//	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--name", "run-xyz",
	//		"--namespace", "test",
	//		"--no-wait",
	//		"--",
	//		"date",
	//	})

	//	require.NoError(t, err)
	//	require.Equal(t, 0, code)
	//})

	//t.Run("WithSpecificExitCode", func(t *testing.T) {
	//	tarball := createTarballWithFiles(t, "test1.tf", "test2.tf")
	//	dest := createTempPath(t)

	//	factory := fake.NewFactory(shellWithoutAnnotation)
	//	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	//	code, err := cmd.Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--name", "run-xyz",
	//		"--namespace", "test",
	//		"--tarball", tarball,
	//		"--path", dest,
	//		"--no-wait",
	//		"--",
	//		"sh", "-c", "exit 101",
	//	})

	//	require.EqualError(t, err, "runner: exit status 101")
	//	require.Equal(t, 101, code)
	//})

	//// Test interaction between client and runner. Client sets annotation on command, runner waits for
	//// annotation to be unset, client unsets annotation, runner returns without error.
	//t.Run("WithAnnotationSetThenUnset", func(t *testing.T) {
	//	tarball := createTarballWithFiles(t, "test1.tf", "test2.tf")
	//	dest := createTempPath(t)

	//	factory := fake.NewFactory(shellWithoutAnnotation)
	//	code, err := newStokCmd(factory, os.Stdout, os.Stderr).Execute([]string{
	//		"runner",
	//		"--kind", "Run",
	//		"--name", "run-xyz",
	//		"--namespace", "test",
	//		"--tarball", tarball,
	//		"--path", dest,
	//		"--",
	//		"/bin/ls", filepath.Join(dest, "test1.tf"),
	//	})
	//	require.Equal(t, 0, code)
	//	require.NoError(t, err)
	//})
}
