package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	expect "github.com/google/goexpect"
	"github.com/stretchr/testify/require"
)

// step invokes command with a pty, expecting the input/output to match the
// batch expectations. Blocks until process has finished.
func step(t *testing.T, name string, args []string, batch []expect.Batcher) error {
	exp, errch, err := expect.SpawnWithArgs(args, 60*time.Second, expect.PartialMatch(true), expect.Tee(nopWriteCloser{t}))
	if err != nil {
		return err
	}

	_, err = exp.ExpectBatch(batch, 60*time.Second)
	if err != nil {
		return err
	}

	return <-errch
}

func createNamespace(t *testing.T, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := client.KubeClient.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		// Only a namespace already exists error is acceptable
		require.True(t, errors.IsAlreadyExists(err))
	}
}

// Delete namespace and wait for it to be deleted
func deleteNamespace(t *testing.T, name string) {
	_ = client.KubeClient.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{})
	err := wait.PollImmediate(time.Second, 60*time.Second, func() (bool, error) {
		if _, err := client.KubeClient.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("waiting for workspace to be deleted: %w", err)
		}
		return false, nil
	})
	require.NoError(t, err)
}

type nopWriteCloser struct {
	t *testing.T
}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	n.t.Log(string(p))
	return len(p), nil
}

func (n nopWriteCloser) Close() error {
	return nil
}
