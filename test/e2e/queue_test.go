package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2E test of queueing functionality
func TestQueue(t *testing.T) {
	// Instantiate etok client
	client, err := etokclient.NewClientCreator().Create(*kubectx)
	require.NoError(t, err)

	// The e2e tests, each composed of multiple steps
	tests := []test{
		{
			name:      "defaults",
			namespace: "e2e-queue",
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
			root := testutil.NewTempDir(t).Root()

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

			t.Run("queue several runs", func(t *testing.T) {
				g, _ := errgroup.WithContext(context.Background())

				// Fire off first run, block the queue for 5 seconds
				g.Go(func() error {
					return step(tt, []string{buildPath, "sh", "uname; sleep 5",
						"--path", root,
						"--context", *kubectx,
					}, []expect.Batcher{
						&expect.BExp{R: `Linux`},
					})
				})

				// Give first run some time to ensure it runs first
				time.Sleep(time.Second)

				// Fire off two further runs in parallel
				for i := 0; i < 3; i++ {
					g.Go(func() error {
						return step(tt, []string{buildPath, "sh", "uname",
							"--path", root,
							"--context", *kubectx,
						}, []expect.Batcher{
							&expect.BExp{R: `Queued: `},
							&expect.BExp{R: `Linux`},
						})
					})
				}

				assert.NoError(t, g.Wait())
			})
		})

		// Delete namespace for each e2e test, ignore any errors
		if !*disableNamespaceDelete {
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), tt.namespace, metav1.DeleteOptions{})
		}
	}
}
