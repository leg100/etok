package backup

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchBucket {
				return nil, fmt.Errorf("%w: %s", ErrBucketNotFound, bucket)
			}
		}
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
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchKey {
				// No backup to restore
				return nil, nil
			}
		}
		return nil, err
	}
	defer resp.Body.Close()

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
