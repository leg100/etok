package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/operator-framework/operator-sdk/pkg/status"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var newWorkspaceReactor = func(cr client.Client, ctx context.Context, key runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
	// Ignore create actions for non-command objs
	ws, ok := obj.(*v1alpha1.Workspace)
	if !ok {
		return obj, nil
	}

	// Mock workspace controller
	ws.Status.Conditions.SetCondition(status.Condition{
		Type:   v1alpha1.ConditionHealthy,
		Status: corev1.ConditionTrue,
	})

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ws.PodName(),
			Namespace: ws.GetNamespace(),
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
	if err := cr.Create(ctx, &pod); err != nil {
		return nil, err
	}
	return ws, nil
}

func TestNewWorkspaceWithoutArgs(t *testing.T) {
	code, err := newStokCmd(fake.NewFactory(), os.Stdout, os.Stderr).Execute([]string{
		"workspace",
		"new",
	})

	require.EqualError(t, err, "accepts 1 arg(s), received 0")
	require.Equal(t, 1, code)
}

func TestNewWorkspaceWithoutFlags(t *testing.T) {
	factory := fake.NewFactory().
		AddReactor("create", newWorkspaceReactor).
		AddReactor("create", func(_ client.Client, _ context.Context, _ runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
			require.Equal(t, "", obj.(*v1alpha1.Workspace).Spec.SecretName)
			require.Equal(t, "", obj.(*v1alpha1.Workspace).Spec.ServiceAccountName)
			return obj, nil
		})

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)
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
	factory := fake.NewFactory(secret, serviceaccount).
		AddReactor("create", newWorkspaceReactor).
		AddReactor("create", func(_ client.Client, _ context.Context, _ runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
			require.Equal(t, "foo", obj.(*v1alpha1.Workspace).Spec.SecretName)
			require.Equal(t, "bar", obj.(*v1alpha1.Workspace).Spec.ServiceAccountName)
			return obj, nil
		})

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
	factory := fake.NewFactory(secret, serviceaccount).
		AddReactor("create", newWorkspaceReactor).
		AddReactor("create", func(_ client.Client, _ context.Context, _ runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
			require.Equal(t, "stok", obj.(*v1alpha1.Workspace).Spec.SecretName)
			require.Equal(t, "stok", obj.(*v1alpha1.Workspace).Spec.ServiceAccountName)
			return obj, nil
		})

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestNewWorkspaceWithSpecificNamespace(t *testing.T) {
	factory := fake.NewFactory().
		AddReactor("create", newWorkspaceReactor)

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
		"--namespace", "test",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)
}

func TestNewWorkspaceWithCacheSettings(t *testing.T) {
	factory := fake.NewFactory().
		AddReactor("create", newWorkspaceReactor).
		AddReactor("create", func(_ client.Client, _ context.Context, _ runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
			require.Equal(t, "999Gi", obj.(*v1alpha1.Workspace).Spec.Cache.Size)
			require.Equal(t, "lumpen-proletariat", obj.(*v1alpha1.Workspace).Spec.Cache.StorageClass)
			return obj, nil
		})

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
}

func TestNewWorkspaceWithContextFlag(t *testing.T) {
	factory := fake.NewFactory().
		AddReactor("create", newWorkspaceReactor)

	var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

	code, err := cmd.Execute([]string{
		"workspace",
		"new",
		"foo",
		"--context", "oz-cluster",
	})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, "oz-cluster", factory.Context)
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
