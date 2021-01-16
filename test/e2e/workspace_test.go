package e2e

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// E2E test of workspace commands (new, list, etc)
func TestWorkspace(t *testing.T) {
	name := "workspace"
	namespace := "e2e-workspace"

	t.Parallel()
	t.Run(name, func(t *testing.T) {
		// Create temp dir of terraform configs and return path to root module
		path := createTerraformConfigs(t)

		t.Run("create namespace", func(t *testing.T) {
			// (Re-)create dedicated namespace for e2e test
			deleteNamespace(t, namespace)
			createNamespace(t, namespace)
		})

		t.Run("create workspace foo", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "new", "foo",
					"--namespace", namespace,
					"--path", path,
					"--context", *kubectx,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", namespace)},
					&expect.BExp{R: "Skipping terraform installation"},
				}))
		})

		t.Run("create workspace bar with custom terraform version", func(t *testing.T) {
			version := "0.12.17"
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "new", "bar",
					"--namespace", namespace,
					"--path", path,
					"--context", *kubectx,
					"--terraform-version", version,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Created workspace %s/bar", namespace)},
					&expect.BExp{R: fmt.Sprintf("Requested terraform version is %s", version)},
					&expect.BExp{R: fmt.Sprintf("Downloading terraform %s", version)},
				}))
		})

		t.Run("list workspaces", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "list",
					"--path", path,
					"--context", *kubectx,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("\\*\t%s/%s\n\t%s/%s", namespace, "bar", namespace, "foo")},
				}))
		})

		t.Run("show current workspace bar", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "show",
					"--path", path,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("%s/%s", namespace, "bar")},
				}))
		})

		t.Run("select workspace foo", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "select", "foo", "--namespace", namespace},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Current workspace now: %s/%s", namespace, "foo")},
				}))
		})

		t.Run("delete workspace foo", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "delete", "foo",
					"--namespace", namespace,
					"--context", *kubectx},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Deleted workspace %s/foo", namespace)},
				}))
		})

		t.Run("delete workspace bar", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "delete", "bar",
					"--namespace", namespace,
					"--context", *kubectx},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Deleted workspace %s/bar", namespace)},
				}))
		})

		t.Run("delete namespace", func(t *testing.T) {
			// Delete namespace for e2e test, ignore any errors
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		})
	})
}
