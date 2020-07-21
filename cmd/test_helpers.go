package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/util"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createTarballWithFiles(t *testing.T, filenames ...string) string {
	path, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	for _, f := range filenames {
		fpath := filepath.Join(path, f)
		ioutil.WriteFile(fpath, []byte{}, 0644)
	}

	// Create test tarball
	buf, err := util.Create(path, filenames)
	require.NoError(t, err)

	// Write tarball to known path
	tarball := filepath.Join(path, "tarball.tar.gz")
	err = ioutil.WriteFile(tarball, buf.Bytes(), 0644)
	require.NoError(t, err)

	return tarball
}

// Create workspace directory and make it the current working dir. Switch back to previous CWD
// when test finishes
func setupEnvironment(t *testing.T, namespace, workspace string) {
	// when test finishes.
	path := createTempPath(t)
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(path))
	t.Cleanup(func() { os.Chdir(previous) })

	require.NoError(t, writeEnvironmentFile(path, namespace, workspace))
}

func createTempPath(t *testing.T) string {
	path, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	t.Cleanup(func() { os.RemoveAll(path) })

	return path
}

var shell = &v1alpha1.Shell{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "stok-shell-xyz",
		Namespace: "test",
	},
	CommandSpec: v1alpha1.CommandSpec{
		Args: []string{"cow", "pig"},
	},
}

func namespaceObj(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func workspaceObj(namespace, name string) *v1alpha1.Workspace {
	return &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: v1alpha1.WorkspaceStatus{
			Conditions: status.Conditions{
				{
					Type:   v1alpha1.ConditionHealthy,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}
