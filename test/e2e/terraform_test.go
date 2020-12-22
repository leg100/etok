package e2e

import (
	"context"
	goctx "context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"cloud.google.com/go/storage"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/env"
	"github.com/stretchr/testify/require"
)

const (
	buildPath     = "../../etok"
	backendBucket = "automatize-tfstate"
	backendPrefix = "e2e"
)

var (
	kubectx                = flag.String("context", "kind-kind", "Kubeconfig context to use for tests")
	disableNamespaceDelete = flag.Bool("disable-namespace-delete", false, "Disable automatic deletion of namespace at end of test")
)

type test struct {
	name       string
	namespace  string
	workspace  string
	pty        bool
	privileged []string
}

func (t *test) tfWorkspace() string {
	return env.TerraformName(t.namespace, t.workspace)
}

// E2E test of terraform commands (plan, apply, etc)
func TestTerraform(t *testing.T) {
	// Instantiate GCS client
	sclient, err := storage.NewClient(goctx.Background())
	require.NoError(t, err)

	// Instantiate etok client
	client, err := etokclient.NewClientCreator().Create(*kubectx)
	require.NoError(t, err)

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

			t.Run("create workspace", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "workspace", "new", "foo",
						"--namespace", tt.namespace,
						"--path", root,
						"--context", *kubectx,
						"--privileged-commands", strings.Join(tt.privileged, ",")},
					[]expect.Batcher{
						&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", tt.namespace)},
					}))
			})

			t.Run("plan", func(t *testing.T) {
				require.NoError(t, step(tt,
					[]string{buildPath, "plan",
						"--path", root,
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
						"--path", root,
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
						"--path", root,
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

func step(t test, args []string, batch []expect.Batcher) error {
	exp, errch, err := expect.SpawnWithArgs(args, 60*time.Second, expect.Tee(nopWriteCloser{os.Stdout}))
	if err != nil {
		return err
	}

	_, err = exp.ExpectBatch(batch, 60*time.Second)
	if err != nil {
		return err
	}

	return <-errch
}

func newNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newSecret(name, creds string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		StringData: map[string]string{
			"GOOGLE_CREDENTIALS": creds,
		},
	}
}

type nopWriteCloser struct {
	f *os.File
}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	return n.f.Write(p)
}

func (n nopWriteCloser) Close() error {
	return nil
}

func exitCodeTest(t *testing.T, err error, wantExitCode int) {
	if exiterr, ok := err.(*exec.ExitError); ok {
		require.Equal(t, wantExitCode, exiterr.ExitCode())
	} else if err != nil {
		require.NoError(t, err)
	} else {
		// got exit code 0; ensures thats whats wanted
		require.Equal(t, wantExitCode, 0)
	}
}
