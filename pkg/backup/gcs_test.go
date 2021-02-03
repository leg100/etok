package backup

import (
	"context"
	"testing"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGCSProvider(t *testing.T) {
	tests := []struct {
		name       string
		bucket     string
		bucketObjs []fakestorage.Object
		secret     *corev1.Secret
	}{
		{
			name:   "Backup",
			bucket: "backups",
			bucketObjs: []fakestorage.Object{
				{
					BucketName: "backups",
				},
			},
			secret: testobj.Secret("default", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Setup up new fake GCS server for each test
			server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
				InitialObjects: tt.bucketObjs,
				Host:           "127.0.0.1",
				Port:           8081,
			})
			require.NoError(t, err)
			defer server.Stop()

			p, err := NewGCSProvider(context.Background(), tt.bucket, server.Client())
			require.NoError(t, err)

			require.NoError(t, p.Backup(context.Background(), tt.secret))
			secret, err := p.Restore(context.Background(), client.ObjectKeyFromObject(tt.secret))

			assert.Equal(t, tt.secret, secret)
		})
	}
}
