package cmd

import (
	"bytes"
	"testing"

	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestListWorkspaces(t *testing.T) {
	ws1 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}
	ws2 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-2",
			Namespace: "dev",
		},
	}

	var factory = fake.NewFactory(ws1, ws2)
	var out = new(bytes.Buffer)
	var cmd = newStokCmd(factory, out, out)

	path := createTempPath(t)
	err := writeEnvironmentFile(path, "default", "workspace-1")
	require.NoError(t, err)

	code, err := cmd.Execute([]string{
		"workspace",
		"list",
		"--path", path,
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, "*\tdefault/workspace-1\n\tdev/workspace-2\n", out.String())
}
