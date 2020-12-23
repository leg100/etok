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

// E2E test of shell command
func TestShell(t *testing.T) {
	// The e2e tests, each composed of multiple steps
	tests := []test{
		{
			name:      "defaults",
			namespace: "e2e-shell",
		},
	}

	// Enumerate e2e tests
	for _, tt := range tests {
		// Delete any GCS backend state beforehand, ignoring any errors
		bkt := sclient.Bucket(backendBucket)
		bkt.Object(fmt.Sprintf("%s/%s_foo.tfstate", backendPrefix, tt.namespace)).Delete(goctx.Background())
		bkt.Object(fmt.Sprintf("%s/%s_foo.tflock", backendPrefix, tt.namespace)).Delete(goctx.Background())

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

			t.Run("create workspace", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "workspace", "new", "foo",
						"--namespace", tt.namespace,
						"--context", *kubectx,
					},
					[]expect.Batcher{
						&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", tt.namespace)},
					}))
			})

			t.Run("single command", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "sh", "uname",
						"--path", root,
						"--context", *kubectx,
					},
					[]expect.Batcher{
						&expect.BExp{R: `Linux`},
					}))
			})

			t.Run("shell session", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "sh",
						"--path", root,
						"--context", *kubectx,
					},
					[]expect.Batcher{
						&expect.BExp{R: `#`},
						&expect.BSnd{S: "uname; exit\n"},
						&expect.BExp{R: `Linux`},
					}))
			})
		})

		// Delete namespace for each e2e test, ignore any errors
		if !*disableNamespaceDelete {
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), tt.namespace, metav1.DeleteOptions{})
		}
	}
}
