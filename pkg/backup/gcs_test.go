package backup

import (
	"context"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

type gcsTestProvider struct {
	*gcsProvider
}

func (p *gcsTestProvider) createProviderWithBuckets(t *testutil.T, providerBucket string, createBuckets ...string) (Provider, error) {
	var initialObjects []fakestorage.Object

	for _, b := range createBuckets {
		initialObjects = append(initialObjects, fakestorage.Object{
			BucketName: b,
		})
	}

	// Setup up fake GCS server
	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
		InitialObjects: initialObjects,
		Host:           "127.0.0.1",
		Port:           8081,
	})
	require.NoError(t, err)
	t.Cleanup(server.Stop)

	return NewGCSProvider(context.Background(), providerBucket, server.Client())
}
