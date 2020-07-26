package cmd

import (
	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
)

func TestTerraform(t *testing.T) {
	for _, kind := range command.CommandKinds {
		t.Run(kind+"WithDefaults", func(t *testing.T) {
			setupEnvironment(t, "default", "default")
			var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default"))
			var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

			code, err := cmd.Execute([]string{
				command.CommandKindToCLI(kind),
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

	t.Run("PlanWithConfigExceedingMaxSize", func(t *testing.T) {
		setupEnvironment(t, "default", "default")

		randBytes := make([]byte, v1alpha1.MaxConfigSize+1)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)

		err = ioutil.WriteFile("toobig.tf", randBytes, 0644)
		require.NoError(t, err)

		var factory = fake.NewFactory(namespaceObj("default"), workspaceObj("default", "default"))
		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--",
			"-input", "false",
		})

		require.Error(t, err)
		require.Equal(t, 1, code)

		plan := factory.Objs[2].(*v1alpha1.Plan)
		require.Equal(t, []string{"-input", "false"}, plan.GetArgs())
	})
}
