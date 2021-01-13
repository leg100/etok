package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test backup and restore of terraform state
func TestBackupRestore(t *testing.T) {
	name := "terraform"
	namespace := "e2e-backup-restore"

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
					"--variables", "suffix=bar",
					"--backup-bucket", backupBucket,
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
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Terraform has been successfully initialized!`},
				}))
		})

		// Run terraform apply just so that we can generate some proper state
		t.Run("apply", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "apply",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "yes\n"},
					&expect.BExp{R: `Apply complete! Resources: 1 added, 0 changed, 0 destroyed.`},
				}))
		})

		// The backup process is asynchronous so give it a little bit of time to
		// complete...
		time.Sleep(time.Second)

		// Check state backup exists
		t.Run("state backup", func(t *testing.T) {
			_, err := sclient.Bucket(backupBucket).Object(fmt.Sprintf("%s/foo.yaml", namespace)).Attrs(context.Background())
			require.NoError(t, err)
		})

		// Delete workspace, which also deletes the state
		t.Run("delete workspace", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "delete", "foo",
					"--namespace", namespace,
					"--context", *kubectx},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Deleted workspace %s/foo", namespace)},
				}))
		})

		// Re-create workspace
		t.Run("create workspace", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "new", "foo",
					"--namespace", namespace,
					"--path", path,
					"--context", *kubectx,
					"--variables", "suffix=bar",
					"--backup-bucket", backupBucket,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", namespace)},
				}))
		})

		// Confirm state has been restored
		_, err := client.KubeClient.CoreV1().Secrets(namespace).Get(context.Background(), "tfstate-default-foo", metav1.GetOptions{})
		assert.NoError(t, err)
	})

	// Delete namespace for each e2e test, ignore any errors
	if !*disableNamespaceDelete {
		_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	}
}
