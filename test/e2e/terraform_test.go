package e2e

import (
	"context"
	goctx "context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// E2E test of terraform commands (plan, apply, etc)
func TestTerraform(t *testing.T) {
	// The e2e tests, each composed of multiple steps
	tests := []test{
		{
			name:      "defaults",
			namespace: "e2e",
			workspace: "foo",
		},
	}

	// Enumerate e2e tests
	for _, tt := range tests {
		// Delete any GCS backend state beforehand, ignoring any errors
		bkt := sclient.Bucket(backendBucket)
		bkt.Object(fmt.Sprintf("%s/%s.tfstate", backendPrefix, tt.tfWorkspace())).Delete(goctx.Background())
		bkt.Object(fmt.Sprintf("%s/%s.tflock", backendPrefix, tt.tfWorkspace())).Delete(goctx.Background())

		// Create namespace for each e2e test
		_, err := client.KubeClient.CoreV1().Namespaces().Create(context.Background(), newNamespace(tt.namespace), metav1.CreateOptions{})
		if err != nil {
			// Only a namespace already exists error is acceptable
			require.True(t, errors.IsAlreadyExists(err))
		}

		t.Parallel()
		t.Run(tt.name, func(t *testing.T) {
			// Create temp dir of terraform configs and set pwd to root module
			createTerraformConfigs(t)

			t.Run("create secret", func(t *testing.T) {
				creds, err := ioutil.ReadFile((os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
				require.NoError(t, err)
				_, err = client.KubeClient.CoreV1().Secrets(tt.namespace).Create(context.Background(), newSecret("etok", string(creds)), metav1.CreateOptions{})
				if err != nil {
					// Only a secret already exists error is acceptable
					require.True(t, errors.IsAlreadyExists(err))
				}
			})

			t.Run("create workspace", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "workspace", "new", "foo",
						"--namespace", tt.namespace,
						"--context", *kubectx,
						"--privileged-commands", strings.Join(tt.privileged, ",")},
					[]expect.Batcher{
						&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", tt.namespace)},
					}))
			})

			t.Run("plan", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "plan",
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
				require.NoError(t, step(tt,
					[]string{buildPath, "apply",
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
				require.NoError(t, step(tt,
					[]string{buildPath, "destroy",
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
				require.NoError(t, step(tt,
					[]string{buildPath, "workspace", "delete", "foo",
						"--namespace", tt.namespace,
						"--context", *kubectx},
					[]expect.Batcher{
						&expect.BExp{R: fmt.Sprintf("Deleted workspace %s/foo", tt.namespace)},
					}))
			})
		})

		// Delete namespace for each e2e test, ignore any errors
		if !*disableNamespaceDelete {
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), tt.namespace, metav1.DeleteOptions{})
		}
	}
}
