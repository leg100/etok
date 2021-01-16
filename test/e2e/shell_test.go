package e2e

import (
	"context"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// E2E test of shell command
func TestShell(t *testing.T) {
	name := "shell"
	namespace := "e2e-shell"

	t.Parallel()
	t.Run(name, func(t *testing.T) {
		// Create temp dir of terraform configs and set pwd to root module
		path := createTerraformConfigs(t)

		t.Run("create namespace", func(t *testing.T) {
			// (Re-)create dedicated namespace for e2e test
			deleteNamespace(t, namespace)
			createNamespace(t, namespace)
		})

		t.Run("create workspace", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "new", "foo",
					"--namespace", namespace,
					"--path", path,
					"--context", *kubectx,
					"--environment-variables", "FOO=bar",
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", namespace)},
				}))
		})

		t.Run("single command", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "sh", "uname",
					"--path", path,
					"--context", *kubectx,
				},
				[]expect.Batcher{
					&expect.BExp{R: `Linux`},
				}))
		})

		t.Run("workspace environment variable", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "sh", "echo $FOO",
					"--path", path,
					"--context", *kubectx,
				},
				[]expect.Batcher{
					&expect.BExp{R: `bar`},
				}))
		})

		t.Run("shell session", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "sh",
					"--path", path,
					"--context", *kubectx,
				},
				[]expect.Batcher{
					&expect.BExp{R: `#`},
					&expect.BSnd{S: "uname; exit\n"},
					&expect.BExp{R: `Linux`},
				}))
		})

		t.Run("delete namespace", func(t *testing.T) {
			// Delete namespace for e2e test, ignore any errors
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		})
	})
}
