package e2e

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/stretchr/testify/require"
)

// E2E test of shell command
func TestShell(t *testing.T) {
	// Instantiate etok client
	client, err := etokclient.NewClientCreator().Create(*kubectx)
	require.NoError(t, err)

	// The e2e tests, each composed of multiple steps
	tests := []test{
		{
			name:      "defaults",
			namespace: "e2e-shell",
		},
	}

	// Enumerate e2e tests
	for _, tt := range tests {
		// Create namespace for each e2e test
		_, err := client.KubeClient.CoreV1().Namespaces().Create(context.Background(), newNamespace(tt.namespace), metav1.CreateOptions{})
		if err != nil {
			// Only a namespace already exists error is acceptable
			require.True(t, errors.IsAlreadyExists(err))
		}

		t.Parallel()
		t.Run(tt.name, func(t *testing.T) {
			// Create terraform configs
			root := createTerraformConfigs(t)

			t.Run("create secret", func(t *testing.T) {
				creds, err := ioutil.ReadFile((os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
				require.NoError(t, err)
				_, err = client.KubeClient.CoreV1().Secrets(tt.namespace).Create(context.Background(), newSecret("etok", string(creds)), metav1.CreateOptions{})
				if err != nil {
					// Only a secret already exists error is acceptable
					require.True(t, errors.IsAlreadyExists(err))
				}
			})

			// This test generates a lock file on the local filesystem (and
			// generally a user is expeted to run terraform init locally in
			// order to do other things like run terraform validate, etc).
			t.Run("run terraform init locally", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{"terraform", "init", "-no-color"},
					[]expect.Batcher{
						&expect.BExp{R: `Terraform has created a lock file .terraform.lock.hcl`},
					}))
			})

			t.Run("create workspace", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "workspace", "new", "foo",
						"--namespace", tt.namespace,
						"--path", root,
						"--context", *kubectx,
					},
					[]expect.Batcher{
						&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", tt.namespace)},
					}))
			})

			// An etok plan also runs `terraform init` and `terraform workspace
			// new` before running `terraform plan`. These steps are necessary
			// in order to complete the workspace setup by creating the
			// workspace's backend state. Only then can the user shell into the
			// pod and run arbitrary terraform commands like plan and apply
			// without terraform complaining the workspace isn't fully setup.
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

			// This test confirms artefacts generated from the previous step
			// have been cached on the persistent volume (and tests that the
			// attachment to the pod tty is functioning).
			t.Run("shell session run terraform plan", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "sh",
						"--context", *kubectx,
					},
					[]expect.Batcher{
						&expect.BExp{R: `/workspace/root # `},
						&expect.BSnd{S: "terraform plan -no-color\n"},
						&expect.BExp{R: `Enter a value:`},
						&expect.BSnd{S: "foo\n"},
						&expect.BExp{R: `Plan: 1 to add, 0 to change, 0 to destroy.`},
						&expect.BExp{R: `/workspace/root # `},
						&expect.BSnd{S: "exit\n"},
					}))
			})

			// This test confirms the user is able to run terraform init locally
			// without error (e.g. about workspace not existing).
			t.Run("run terraform init locally again", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{"terraform", "init", "-no-color"},
					[]expect.Batcher{
						&expect.BExp{R: `Reusing previous version of hashicorp/random from the dependency lock file`},
					}))
			})
		})

		// Delete namespace for each e2e test, ignore any errors
		if !*disableNamespaceDelete {
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), tt.namespace, metav1.DeleteOptions{})
		}
	}
}
