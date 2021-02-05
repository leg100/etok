package backup

import (
	"context"
	"net/http/httptest"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

type s3TestProvider struct {
	*s3Provider
}

func (p *s3TestProvider) createProviderWithBuckets(t *testutil.T, providerBucket string, createBuckets ...string) (Provider, error) {
	// fake s3
	backend := s3mem.New()
	faker := gofakes3.New(backend)
	ts := httptest.NewServer(faker.Server())
	t.Cleanup(ts.Close)

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

	for _, b := range createBuckets {
		_, err = client.CreateBucket(&s3.CreateBucketInput{
			Bucket: aws.String(b),
		})
		require.NoError(t, err)
	}

	return NewS3Provider(context.Background(), providerBucket, cfg)
}
