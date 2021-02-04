package backup

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestS3Provider(t *testing.T) {
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
	sess, err := session.NewSession(cfg)
	require.NoError(t, err)

	client := s3.New(sess)

	_, err = client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String("backups-bucket"),
	})
	require.NoError(t, err)

	p, err := NewS3Provider(context.Background(), "backups-bucket", cfg)
	require.NoError(t, err)

	secret := testobj.Secret("default", "tfstate-default-workspace-1")

	// Assert that backed up secret matches restored secret
	require.NoError(t, p.Backup(context.Background(), secret))
	restored, err := p.Restore(context.Background(), runtimeclient.ObjectKeyFromObject(secret))
	assert.Equal(t, secret, restored)
}
