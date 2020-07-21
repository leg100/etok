package cmd

import (
	"os"
	"testing"

	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
)

func TestNewWorkspaceWithoutArgs(t *testing.T) {
	var cmd = newStokCmd(fake.NewFactory(), os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
	})

	require.EqualError(t, err, "accepts 1 arg(s), received 0")
	require.Equal(t, 1, code)
}

func TestNewWorkspaceWithoutFlags(t *testing.T) {
	factory := fake.NewFactory()
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	ws := factory.Objs[0].(*v1alpha1.Workspace)
	require.NoError(t, err)
	require.Equal(t, "", ws.Spec.ServiceAccountName)
	require.Equal(t, "stok", ws.Spec.SecretName)
}

func TestNewWorkspaceWithSpecificNamespace(t *testing.T) {
	factory := fake.NewFactory()
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
		"--namespace", "test",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	ws := factory.Objs[0].(*v1alpha1.Workspace)
	require.NoError(t, err)
	require.Equal(t, "test", ws.GetNamespace())
}

func TestNewWorkspaceWithNoSecret(t *testing.T) {
	factory := fake.NewFactory()
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
		"--no-secret",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	ws := factory.Objs[0].(*v1alpha1.Workspace)
	require.NoError(t, err)
	require.Equal(t, "", ws.Spec.SecretName)
}

func TestNewWorkspaceWithCacheSettings(t *testing.T) {
	factory := fake.NewFactory()
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
		"--size", "999Gi",
		"--storage-class", "lumpen-proletariat",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	ws := factory.Objs[0].(*v1alpha1.Workspace)
	require.NoError(t, err)
	require.Equal(t, "999Gi", ws.Spec.Cache.Size)
	require.Equal(t, "lumpen-proletariat", ws.Spec.Cache.StorageClass)
}

//func TestNewWorkspaceWithTimeoutError(t *testing.T) {
//	factory := fake.NewFactory()
//	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)
//
//	code, err := cmd.Execute([]string{
//		"workspace",
//		"new",
//		"foo",
//		"--service-account", "non-existent",
//	})
//	require.Equal(t, 1, code)
//	require.Error(t, err)
//	require.Equal(t, WorkspaceTimeoutErr, err)
//}
