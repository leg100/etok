package cmd

import (
	"os"
	"testing"

	"github.com/leg100/stok/api/run"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTerraform(t *testing.T) {
	podReadyAndRunning := func(namespace, kind string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fake.GenerateName(kind),
				Namespace: namespace,
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
	}

	cmd := func(namespace, cmd string) *v1alpha1.Run {
		run := &v1alpha1.Run{}
		run.SetNamespace(namespace)
		run.SetName(fake.GenerateName(cmd))
		run.SetPhase(v1alpha1.RunPhaseSync)
		return run
	}

	for _, tfcmd := range run.TerraformCommands {
		t.Run(tfcmd+"WithDefaults", func(t *testing.T) {
			setupEnvironment(t, "default", "default")
			var factory = fake.NewFactory(
				workspaceObj("default", "default"),
				cmd("default", tfcmd),
				podReadyAndRunning("default", tfcmd))

			var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

			code, err := cmd.Execute([]string{
				tfcmd,
				"--debug",
			})

			require.NoError(t, err)
			require.Equal(t, 0, code)
		})

	}

	t.Run("PlanWithSpecificNamespaceAndWorkspace", func(t *testing.T) {
		setupEnvironment(t, "foo", "bar")
		var factory = fake.NewFactory(
			workspaceObj("foo", "bar"),
			cmd("foo", "plan"),
			podReadyAndRunning("foo", "plan"))

		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--debug",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("PlanWithSpecificNamespaceAndWorkspaceFlags", func(t *testing.T) {
		// Set environment to be default/default, to be overridden by flags below
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(
			workspaceObj("foo", "bar"),
			cmd("foo", "plan"),
			podReadyAndRunning("foo", "plan"))

		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--debug",
			"--namespace", "foo",
			"--workspace", "bar",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("PlanWithFlag", func(t *testing.T) {
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(
			workspaceObj("default", "default"),
			cmd("default", "plan"),
			podReadyAndRunning("default", "plan"))

		var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

		code, err := cmd.Execute([]string{
			"plan",
			"--",
			"-input", "false",
		})

		require.NoError(t, err)
		require.Equal(t, 0, code)
	})

	t.Run("PlanWithContextFlag", func(t *testing.T) {
		setupEnvironment(t, "default", "default")
		var factory = fake.NewFactory(
			workspaceObj("default", "default"),
			cmd("default", "plan"),
			podReadyAndRunning("default", "plan"))

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
