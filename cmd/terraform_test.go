package cmd

import (
	"os"
	"testing"

	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
)

func TestTerraform(t *testing.T) {
	for _, kind := range v1alpha1.CommandKinds {
		t.Run(kind+"WithDefaults", func(t *testing.T) {
			setupEnvironment(t, "default", "default")
			var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default"))
			var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

			code, err := cmd.Execute([]string{
				v1alpha1.CommandKindToCLI(kind),
			})

			require.NoError(t, err)
			require.Equal(t, 0, code)
			// Namespace, workspace, command, configmap, and pod
			require.Equal(t, 5, len(factory.Objs))
		})

	}

	t.Run("PlanWithSpecificNamespaceAndWorkspace", func(t *testing.T) {
		setupEnvironment(t, "foo", "bar")
		var factory = fake.NewFactory(namespaceObj("foo"), workspaceObj("foo", "bar"))
		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
		// Namespace, workspace, command, configmap, and pod
		require.Equal(t, 5, len(factory.Objs))
	})

	t.Run("PlanWithSpecificNamespaceAndWorkspaceFlags", func(t *testing.T) {
		// Set environment to be default/default, to be overridden by flags below
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(namespaceObj("foo"), workspaceObj("foo", "bar"))
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
		var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default"))
		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--",
			"-input", "false",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)

		plan := factory.Objs[2].(*v1alpha1.Plan)
		require.Equal(t, []string{"-input", "false"}, plan.GetArgs())
	})
}
