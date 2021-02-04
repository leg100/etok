package backup

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/fsouza/fake-gcs-server/fakestorage"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestS3Provider(t *testing.T) {
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
			// fake s3
			backend := s3mem.New()
			faker := gofakes3.New(backend)
			ts := httptest.NewServer(faker.Server())
			defer ts.Close()

			// configure S3 client
			cfg := &aws.Config{
				Credentials:      credentials.NewStaticCredentials("YOUR-ACCESSKEYID", "YOUR-SECRETACCESSKEY", ""),
				Endpoint:         aws.String(ts.URL),
				Region:           aws.String("eu-central-1"),
				DisableSSL:       aws.Bool(true),
				S3ForcePathStyle: aws.Bool(true),
			}

			if tt.bucket != "" {
				sess, err := session.NewSession(cfg)
				require.NoError(t, err)

				client := s3.New(sess)

				cparams := &s3.CreateBucketInput{
					Bucket: aws.String(tt.bucket),
				}

				// Create a new bucket using the CreateBucket call.
				_, err = client.CreateBucket(cparams)
				require.NoError(t, err)
			}

			p, err := NewS3Provider(context.Background(), tt.bucket, cfg)
			require.NoError(t, err)

			require.NoError(t, p.Backup(context.Background(), tt.secret))
			secret, err := p.Restore(context.Background(), client.ObjectKeyFromObject(tt.secret))

			assert.Equal(t, tt.secret, secret)
		})
	}
}
