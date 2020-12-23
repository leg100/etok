package e2e

import (
	"context"
	goctx "context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// E2E test of terraform commands (plan, apply, etc)
func TestTerraform(t *testing.T) {
	name := "terraform"
	namespace := "e2e-terraform"

	// Delete any GCS backend state beforehand, ignoring any errors
	bkt := sclient.Bucket(backendBucket)
	bkt.Object(fmt.Sprintf("%s/%s_foo.tfstate", backendPrefix, namespace)).Delete(goctx.Background())
	bkt.Object(fmt.Sprintf("%s/%s_foo.tflock", backendPrefix, namespace)).Delete(goctx.Background())

	// Create dedicated namespace for e2e test
	createNamespace(t, namespace)

	t.Parallel()
	t.Run(name, func(t *testing.T) {
		// Create temp dir of terraform configs and set pwd to root module
		path := createTerraformConfigs(t)

		t.Run("create secret", func(t *testing.T) {
			// Get google cloud creds from env var
			creds, err := ioutil.ReadFile((os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
			require.NoError(t, err)
			_, err = client.KubeClient.CoreV1().Secrets(namespace).Create(context.Background(), newSecret("etok", string(creds)), metav1.CreateOptions{})
			if err != nil {
				// Only a secret already exists error is acceptable
				require.True(t, errors.IsAlreadyExists(err))
			}
		})

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

		//time.Sleep(10 * time.Second)

		t.Run("plan", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "plan",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-no-color"},
				[]expect.Batcher{
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "foo\n"},
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
					&expect.BSnd{S: "foo\n"},
					&expect.BExp{R: `Enter a value:`},
					&expect.BSnd{S: "yes\n"},
					&expect.BExp{R: `Apply complete! Resources: 1 added, 0 changed, 0 destroyed.`},
				}))
		})

		t.Run("destroy", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "destroy",
					"--path", path,
					"--context", *kubectx, "--",
					"-input=true",
					"-var", "suffix=foo",
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
