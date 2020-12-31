package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// E2E test of terraform commands (plan, apply, etc)
func TestTerraform(t *testing.T) {
	name := "terraform"
	namespace := "e2e-terraform"

	// Create dedicated namespace for e2e test
	createNamespace(t, namespace)

	t.Parallel()
	t.Run(name, func(t *testing.T) {
		// Create temp dir of terraform configs and set pwd to root module
		path := createTerraformConfigs(t)

		t.Run("create workspace", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "new", "foo",
					"--namespace", namespace,
					"--path", path,
					"--context", *kubectx,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", namespace)},
				}))
		})

		t.Run("init", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "init",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Terraform has been successfully initialized!`},
				}))

			// The init command should create a local lock file
			require.FileExists(t, filepath.Join(path, ".terraform.lock.hcl"))
		})

		t.Run("plan", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "plan",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "bar\n"},
					&expect.BExp{R: `Plan: 1 to add, 0 to change, 0 to destroy.`},
				}))
		})

		t.Run("apply", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "apply",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "bar\n"},
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "yes\n"},
					&expect.BExp{R: `Apply complete! Resources: 1 added, 0 changed, 0 destroyed.`},
				}))
		})

		t.Run("output", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "output",
					"--path", path,
					"--context", *kubectx, "--",
				},
				[]expect.Batcher{
					&expect.BExp{R: `random_string = "[0-9a-f]{4}-bar-e2e-terraform-foo"`},
				}))
		})

		t.Run("destroy", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "destroy",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-var", "suffix=bar",
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "yes\n"},
					&expect.BExp{R: `Destroy complete! Resources: 1 destroyed.`},
				}))
		})

		t.Run("delete workspace", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "delete", "foo",
					"--namespace", namespace,
					"--context", *kubectx},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Deleted workspace %s/foo", namespace)},
				}))
		})
	})

	// Delete namespace for each e2e test, ignore any errors
	if !*disableNamespaceDelete {
		_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	}
}
