package launcher

import (
	"context"
	"testing"
	"time"

	"github.com/leg100/stok/pkg/k8s/stokclient/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestCreateRun(t *testing.T) {
	tc := &Launcher{
		Namespace:     "default",
		Workspace:     "default",
		Args:          []string{},
		TimeoutClient: time.Minute,
		TimeoutQueue:  time.Minute,
	}

	// Populate fake stok client with relevant objects
	client := fake.NewSimpleClientset()
	plan, err := tc.createRun(context.Background(), client, "run-12345", "run-12345")
	require.NoError(t, err)

	require.Equal(t, 0, len(plan.GetArgs()))
}

func TestCreateConfigMap(t *testing.T) {
	tc := &Launcher{
		Namespace: "default",
		Workspace: "default",
	}

	client := kfake.NewSimpleClientset()

	// TODO: create real tarball
	tarball := make([]byte, 1024)

	err := tc.createConfigMap(context.Background(), client, tarball, "run-12345", "config.tar.gz")
	require.NoError(t, err)

	cfg, err := client.CoreV1().ConfigMaps(tc.Namespace).Get(context.Background(), "run-12345", metav1.GetOptions{})
	require.NoError(t, err)

	if cfg.GetName() != "run-12345" {
		t.Errorf("want run-12345, got %s\n", cfg.GetName())
	}
}
