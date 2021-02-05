package backup

import (
	"context"
	"errors"
	"testing"

	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// testProvider is a provider plus the ability to create provider as well create
// buckets
type testProvider interface {
	Provider
	createProviderWithBuckets(*testutil.T, string, ...string) (Provider, error)
}

var (
	// testProviders is a collection of implementations of testProvider to be
	// tested
	testProviders = map[string]testProvider{
		"gcs": &gcsTestProvider{},
		"s3":  &s3TestProvider{},
	}
)

func TestProviders(t *testing.T) {
	tests := []struct {
		name           string
		providerBucket string
		createBuckets  []string
		backup         bool
		restore        bool
		wantSecret     *corev1.Secret
		err            error
	}{
		{
			name:           "backup-restore",
			backup:         true,
			restore:        true,
			wantSecret:     testobj.Secret("default", "tfstate-default-workspace-1"),
			providerBucket: "backups-bucket",
			createBuckets:  []string{"backups-bucket"},
		},
		{
			name:           "nothing to restore",
			restore:        true,
			providerBucket: "backups-bucket",
			createBuckets:  []string{"backups-bucket"},
		},
		{
			name:           "missing bucket",
			providerBucket: "backups-bucket",
			err:            ErrBucketNotFound,
		},
	}

	for _, tt := range tests {
		for providerName, tp := range testProviders {
			testutil.Run(t, tt.name+"/"+providerName, func(t *testutil.T) {
				p, err := tp.createProviderWithBuckets(t, tt.providerBucket, tt.createBuckets...)
				if !assert.True(t, errors.Is(err, tt.err)) {
					t.Errorf("no error in %v's chain matches %v", err, tt.err)
				}
				if err != nil {
					return
				}

				secret := testobj.Secret("default", "tfstate-default-workspace-1")

				// Assert that backed up secret matches restored secret
				if tt.backup {
					require.NoError(t, p.Backup(context.Background(), secret))
				}

				if tt.restore {
					restored, err := p.Restore(context.Background(), client.ObjectKeyFromObject(secret))
					require.NoError(t, err)
					assert.Equal(t, tt.wantSecret, restored)
				}
			})
		}
	}

}
