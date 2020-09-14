package cmd

import (
	"os"
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewWorkspace(t *testing.T) {
	workspace := func(namespace, name string, queue ...string) *v1alpha1.Workspace {
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
				Queue: queue,
			},
		}
	}

	pod := func(namespace, name string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "workspace-" + name,
				Namespace: namespace,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
				InitContainerStatuses: []corev1.ContainerStatus{
					{
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{},
						},
					},
				},
			},
		}
	}

	t.Run("WithoutArgs", func(t *testing.T) {
		factory := fake.NewFactory(
			workspace("default", "default"),
			pod("default", "xxx"),
		)
		code, err := newStokCmd(factory, os.Stdout, os.Stderr).Execute([]string{
			"workspace",
			"new",
		})

		require.EqualError(t, err, "accepts 1 arg(s), received 0")
		require.Equal(t, 1, code)
	})

	t.Run("WithoutFlags", func(t *testing.T) {
		factory := fake.NewFactory(
			workspace("default", "foo"),
			pod("default", "foo"),
		)
		cmd := newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"workspace",
			"new",
			"foo",
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("WithUserSuppliedSecretAndServiceAccount", func(t *testing.T) {
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
		factory := fake.NewFactory(
			secret,
			serviceaccount,
			workspace("default", "foo"),
			pod("default", "foo"),
		)
		cmd := newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"workspace",
			"new",
			"foo",
			"--secret", "foo",
			"--service-account", "bar",
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("WithDefaultSecretAndServiceAccount", func(t *testing.T) {
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
		factory := fake.NewFactory(
			secret,
			serviceaccount,
			workspace("default", "foo"),
			pod("default", "foo"),
		)
		cmd := newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"workspace",
			"new",
			"foo",
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("WithSpecificNamespace", func(t *testing.T) {
		factory := fake.NewFactory(
			workspace("test", "foo"),
			pod("test", "foo"),
		)
		cmd := newStokCmd(factory, os.Stdout, os.Stderr)
		code, err := cmd.Execute([]string{
			"workspace",
			"new",
			"foo",
			"--namespace", "test",
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("WithCacheSettings", func(t *testing.T) {
		factory := fake.NewFactory(
			workspace("default", "foo"),
			pod("default", "foo"),
		)
		cmd := newStokCmd(factory, os.Stdout, os.Stderr)
		code, err := cmd.Execute([]string{
			"workspace",
			"new",
			"foo",
			"--size", "999Gi",
			"--storage-class", "lumpen-proletariat",
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("WithContextFlag", func(t *testing.T) {
		factory := fake.NewFactory(
			workspace("default", "foo"),
			pod("default", "foo"),
		)

		cmd := newStokCmd(factory, os.Stdout, os.Stderr)
		code, err := cmd.Execute([]string{
			"workspace",
			"new",
			"foo",
			"--context", "oz-cluster",
		})
		require.NoError(t, err)
		require.Equal(t, 0, code)
		require.Equal(t, "oz-cluster", factory.Context)
	})
}
