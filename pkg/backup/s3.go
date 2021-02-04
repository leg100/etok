package backup

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type s3Provider struct {
	bucket string
	client *s3.S3
}

func NewS3Provider(ctx context.Context, bucket string, cfg *aws.Config) (Provider, error) {
	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, err
	}

	svc := s3.New(sess)

	// Check bucket exists
	if _, err := svc.GetBucketAclWithContext(ctx, &s3.GetBucketAclInput{Bucket: &bucket}); err != nil {
		return nil, err
	}

	return &s3Provider{
		bucket: bucket,
		client: svc,
	}, nil
}

func (p *s3Provider) Backup(ctx context.Context, secret *corev1.Secret) error {
	// Marshal state file first to json then to yaml
	y, err := yaml.Marshal(secret)
	if err != nil {
		return err
	}

	_, err = p.client.PutObject(&s3.PutObjectInput{
		Body:   bytes.NewReader(y),
		Bucket: aws.String(p.bucket),
		Key:    aws.String(path(client.ObjectKeyFromObject(secret))),
	})

	if err != nil {
		return err
	}

	return nil
}

func (p *s3Provider) Restore(ctx context.Context, key client.ObjectKey) (*corev1.Secret, error) {
	var secret corev1.Secret

	resp, err := p.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(path(key)),
	})
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Unmarshal state file into secret obj
	if err := yaml.Unmarshal(buf, &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

func init() {
	providerToFlagsMaker["s3"] = func() flags { return &s3Flags{} }
}

type s3Flags struct {
	bucket string
	region string
}

func (f *s3Flags) addToFlagSet(fs *pflag.FlagSet) {
	fs.StringVar(&f.bucket, "s3-bucket", "", "Specify s3 bucket for terraform state backups")
	fs.StringVar(&f.region, "s3-region", "", "Specify s3 region for terraform state backups")
}

func (f *s3Flags) createProvider(ctx context.Context) (Provider, error) {
	return NewS3Provider(ctx, f.bucket, &aws.Config{Region: aws.String(f.region)})
}

func (f *s3Flags) validate() error {
	if f.bucket == "" {
		return fmt.Errorf("%w: missing s3 bucket name", ErrInvalidConfig)
	}
	if f.region == "" {
		return fmt.Errorf("%w: missing s3 region name", ErrInvalidConfig)
	}
	return nil
}
