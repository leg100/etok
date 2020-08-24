package cmd

import (
	"crypto/rand"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s/fake"
	"github.com/leg100/stok/scheme"
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

	cmd := func(namespace, kind string) command.Interface {
		obj, err := command.NewCommandFromGVK(scheme.Scheme, v1alpha1.SchemeGroupVersion.WithKind(strings.Title(kind)))
		require.NoError(t, err)

		obj.SetNamespace(namespace)
		obj.SetName(fake.GenerateName(kind))
		obj.SetPhase(v1alpha1.CommandPhaseActive)
		return obj
	}

	for _, kind := range command.CommandKinds {
		t.Run(kind+"WithDefaults", func(t *testing.T) {
			setupEnvironment(t, "default", "default")
			var factory = fake.NewFactory(
				workspaceObj("default", "default"),
				cmd("default", kind),
				podReadyAndRunning("default", kind))

			var cmd = newStokCmd(factory, os.Stdout, os.Stderr)

			code, err := cmd.Execute([]string{
				command.CommandKindToCLI(kind),
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

	t.Run("PlanWithConfigExceedingMaxSize", func(t *testing.T) {
		setupEnvironment(t, "default", "default")

		randBytes := make([]byte, v1alpha1.MaxConfigSize+1)
		_, err := rand.Read(randBytes)
		require.NoError(t, err)

		err = ioutil.WriteFile("toobig.tf", randBytes, 0644)
		require.NoError(t, err)

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

		require.Error(t, err)
		require.Equal(t, 1, code)
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
