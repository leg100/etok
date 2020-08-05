package cmd

import (
	"context"
	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestTerraform(t *testing.T) {
	var createPodReactor = func(cr client.Client, ctx context.Context, key runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
		// Ignore create actions for non-command objs
		if _, ok := obj.(command.Interface); !ok {
			return obj, nil
		}

		pod := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}
		if err := cr.Create(ctx, &pod); err != nil {
			return nil, err
		}
		return obj, nil
	}

	for _, kind := range command.CommandKinds {
		t.Run(kind+"WithDefaults", func(t *testing.T) {
			setupEnvironment(t, "default", "default")
			var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default")).
				AddReactor("create", createPodReactor)

			var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

			code, err := cmd.Execute([]string{
				command.CommandKindToCLI(kind),
			})

			require.NoError(t, err)
			require.Equal(t, 0, code)
		})

	}

	t.Run("PlanWithSpecificNamespaceAndWorkspace", func(t *testing.T) {
		setupEnvironment(t, "foo", "bar")
		var factory = fake.NewFactory(namespaceObj("foo"), workspaceObj("foo", "bar")).
			AddReactor("create", createPodReactor)
		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("PlanWithSpecificNamespaceAndWorkspaceFlags", func(t *testing.T) {
		// Set environment to be default/default, to be overridden by flags below
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(namespaceObj("foo"), workspaceObj("foo", "bar")).
			AddReactor("create", createPodReactor)
		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--namespace", "foo",
			"--workspace", "bar",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("PlanWithFlag", func(t *testing.T) {
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default")).
			AddReactor("create", createPodReactor).
			AddReactor("create", func(_ client.Client, _ context.Context, _ runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
				if cmd, ok := obj.(command.Interface); ok {
					require.Equal(t, []string{"-input", "false"}, cmd.GetArgs())
				}
				return obj, nil
			})

		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--",
			"-input", "false",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("PlanWithConfigExceedingMaxSize", func(t *testing.T) {
		setupEnvironment(t, "default", "default")

		randBytes := make([]byte, v1alpha1.MaxConfigSize+1)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)

		err = ioutil.WriteFile("toobig.tf", randBytes, 0644)
		require.NoError(t, err)

		// test whether plan resource is deleted
		var deleted bool
		var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default")).
			AddReactor("create", createPodReactor).
			AddReactor("delete", func(_ client.Client, _ context.Context, _ runtimeclient.ObjectKey, obj runtime.Object) (runtime.Object, error) {
				deleted = true
				return obj, nil
			})

		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--",
			"-input", "false",
		})

		require.Error(t, err)
		require.Equal(t, 1, code)
		require.True(t, deleted)
	})

	t.Run("PlanWithContextFlag", func(t *testing.T) {
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default")).
			AddReactor("create", createPodReactor)
		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--context", "oz-cluster",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
		require.Equal(t, "oz-cluster", factory.Context)
	})
}
