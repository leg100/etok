package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	expect "github.com/google/goexpect"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2E test of queueing functionality
func TestQueue(t *testing.T) {
	name := "queueing"
	namespace := "e2e-queue"

	t.Parallel()
	t.Run(name, func(t *testing.T) {
		// Change into temp dir
		path := testutil.NewTempDir(t).Root()

		t.Run("create namespace", func(t *testing.T) {
			// (Re-)create dedicated namespace for e2e test
			deleteNamespace(t, namespace)
			createNamespace(t, namespace)
		})

		t.Run("create workspace", func(t *testing.T) {
			require.NoError(t, step(t, name,
				[]string{buildPath, "workspace", "new", "foo",
					"--namespace", namespace,
					"--context", *kubectx,
					"--path", path,
				},
				[]expect.Batcher{
					&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", namespace)},
				}))
		})

		t.Run("queue several runs", func(t *testing.T) {
			g, _ := errgroup.WithContext(context.Background())

			// Fire off first run, block the queue for 5 seconds
			g.Go(func() error {
				return step(t, name, []string{buildPath, "sh", "uname; sleep 5",
					"--context", *kubectx,
					"--path", path,
				}, []expect.Batcher{
					&expect.BExp{R: `Linux`},
				})
			})

			// Give first run some time to ensure it runs first
			time.Sleep(time.Second)

			// Fire off two further runs in parallel
			for i := 0; i < 3; i++ {
				g.Go(func() error {
					return step(t, name, []string{buildPath, "sh", "uname",
						"--context", *kubectx,
						"--path", path,
					}, []expect.Batcher{
						&expect.BExp{R: `Queued: `},
						&expect.BExp{R: `Linux`},
					})
				})
			}

			assert.NoError(t, g.Wait())
		})

		t.Run("delete namespace", func(t *testing.T) {
			// Delete namespace for e2e test, ignore any errors
			_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
		})
	})
}
