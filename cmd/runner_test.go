package cmd

import (
	"bytes"
	"context"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/leg100/stok/testutil"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestRunner(t *testing.T) {
	shellWithoutAnnotation := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "run-xyz",
			Namespace: "test",
		},
		RunSpec: v1alpha1.RunSpec{
			Command: "sh",
			Args:    []string{"cow", "pig"},
		},
	}

	tests := []struct {
		name     string
		args     []string
		envs     map[string]string
		stokObjs []runtime.Object
		err      string
		code     int
	}{
		{
			name:     "WithoutKind",
			args:     []string{"runner"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
			err:      "runner: invalid kind: ",
			code:     1,
		},
		{
			name:     "WithIncorrectKind",
			args:     []string{"runner", "--kind", "InvalidKind"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
			err:      "runner: invalid kind: InvalidKind",
			code:     1,
		},
		{
			name:     "WithIncorrectTarball",
			args:     []string{"runner", "--kind", "Run", "--tarball", "bad-tarball-zzz.tar.gz"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
			err:      "runner: open bad-tarball-zzz.tar.gz: no such file or directory",
			code:     1,
		},
		{
			name:     "WithWaitDisabled",
			args:     []string{"runner", "--kind", "Run", "--no-wait", "--", "uname"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
		},
		{
			name:     "WithEnvVar",
			args:     []string{"runner", "--no-wait", "--", "uname"},
			envs:     map[string]string{"STOK_KIND": "Run"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
		},
		{
			name:     "WithTarball",
			args:     []string{"runner", "--kind", "Run", "--name", "run-xyz", "--namespace", "test", "--tarball", "tarball.tar.gz", "--path", ".", "--no-wait", "--", "/bin/ls", "test1.tf"},
			envs:     map[string]string{"STOK_KIND": "Run"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
		},
		{
			name:     "WithTarballArgOverridingEnvVar",
			args:     []string{"runner", "--kind", "Run", "--name", "run-xyz", "--namespace", "test", "--tarball", "tarball.tar.gz", "--path", ".", "--no-wait", "--", "/bin/ls", "test1.tf"},
			envs:     map[string]string{"STOK_TARBALL": "doesnotexist.tar.gz"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
		},
		{
			name:     "WithSpecificExitCode",
			args:     []string{"runner", "--kind", "Run", "--name", "run-xyz", "--namespace", "test", "--tarball", "tarball.tar.gz", "--path", ".", "--no-wait", "--", "sh", "-c", "exit 101"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
			err:      "runner: exit status 101",
			code:     101,
		},
		{
			name:     "WithWaitEnabled",
			args:     []string{"runner", "--kind", "Run", "--name", "run-xyz", "--namespace", "test", "--tarball", "tarball.tar.gz", "--path", ".", "--", "echo", "testing 123"},
			stokObjs: []runtime.Object{shellWithoutAnnotation},
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			t.NewTempDir().Chdir()

			t.SetEnvs(tt.envs)

			createTarballWithFiles(t, "tarball.tar.gz", "test1.tf")

			// Populate fake stok client with relevant objects
			fakeStokClient := fake.NewSimpleClientset(tt.stokObjs...)
			t.Override(&k8s.StokClient, func() (stokclient.Interface, error) {
				return fakeStokClient, nil
			})

			out := new(bytes.Buffer)
			code, err := ExecWithExitCode(context.Background(), tt.args, out, out)

			if tt.err != "" {
				require.EqualError(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.code, code)
		})
	}
}

func createTarballWithFiles(t *testutil.T, name string, filenames ...string) {
	path := t.NewTempDir().Root()

	// Create dummy zero-sized files to be included in archive
	for _, f := range filenames {
		fpath := filepath.Join(path, f)
		ioutil.WriteFile(fpath, []byte{}, 0644)
	}

	// Create test tarball
	tar, err := archive.Create(path)
	require.NoError(t, err)

	// Write tarball to current path
	err = ioutil.WriteFile(name, tar, 0644)
	require.NoError(t, err)
}
