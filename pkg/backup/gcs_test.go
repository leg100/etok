package backup

import (
	"context"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGCSProvider(t *testing.T) {
	// Setup up fake GCS server
	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
		InitialObjects: []fakestorage.Object{
			{
				BucketName: "backups-bucket",
			},
		},
		Host: "127.0.0.1",
		Port: 8081,
	})
	require.NoError(t, err)
	defer server.Stop()

	p, err := NewGCSProvider(context.Background(), "backups-bucket", server.Client())
	require.NoError(t, err)

	secret := testobj.Secret("default", "tfstate-default-workspace-1")

	// Assert that backed up secret matches restored secret
	require.NoError(t, p.Backup(context.Background(), secret))
	restored, err := p.Restore(context.Background(), client.ObjectKeyFromObject(secret))
	assert.Equal(t, secret, restored)
}
