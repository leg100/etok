package cmd

import (
	"os"
	"testing"

	v1alpha1types "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteWorkspace(t *testing.T) {
	ws1 := &v1alpha1types.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: "default",
		},
	}

	factory := fake.NewFactory(ws1)
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)
	code, err := cmd.Execute([]string{
		"workspace",
		"delete",
		"workspace-1",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)
}
