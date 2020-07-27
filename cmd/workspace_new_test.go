package cmd

import (
	"os"
	"testing"

	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	require.Equal(t, "", ws.Spec.SecretName)
}

func TestNewWorkspaceWithUserSuppliedSecretAndServiceAccount(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
	}
	serviceaccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bar",
			Namespace: "default",
		},
	}
	factory := fake.NewFactory(secret, serviceaccount)
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
		"--secret", "foo",
		"--service-account", "bar",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	ws := factory.Objs[2].(*v1alpha1.Workspace)
	require.NoError(t, err)
	require.Equal(t, "foo", ws.Spec.SecretName)
	require.Equal(t, "bar", ws.Spec.ServiceAccountName)
}

func TestNewWorkspaceWithDefaultSecretAndServiceAccount(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stok",
			Namespace: "default",
		},
	}
	serviceaccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stok",
			Namespace: "default",
		},
	}
	factory := fake.NewFactory(secret, serviceaccount)
	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)

	ws := factory.Objs[2].(*v1alpha1.Workspace)
	require.NoError(t, err)
	require.Equal(t, "stok", ws.Spec.SecretName)
	require.Equal(t, "stok", ws.Spec.ServiceAccountName)
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
