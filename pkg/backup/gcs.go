package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

type gcsProvider struct {
	bucket string
	client *storage.Client
}

func NewGCSProvider(ctx context.Context, bucket string, client *storage.Client) (Provider, error) {
	if client == nil {
		var err error
		client, err = storage.NewClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Check bucket exists
	bh := client.Bucket(bucket)
	_, err := bh.Attrs(ctx)
	if err != nil {
		return nil, err
	}

	return &gcsProvider{
		bucket: bucket,
		client: client,
	}, nil
}

func (p *gcsProvider) Backup(ctx context.Context, secret *corev1.Secret) error {
	bh := p.client.Bucket(p.bucket)
	_, err := bh.Attrs(ctx)
	if err != nil {
		return err
	}

	oh := bh.Object(path(client.ObjectKeyFromObject(secret)))

	// Marshal state file first to json then to yaml
	y, err := yaml.Marshal(secret)
	if err != nil {
		return err
	}

	// Copy state file to GCS
	owriter := oh.NewWriter(ctx)
	_, err = io.Copy(owriter, bytes.NewBuffer(y))
	if err != nil {
		return err
	}

	if err := owriter.Close(); err != nil {
		return err
	}

	return nil
}

func (p *gcsProvider) Restore(ctx context.Context, key client.ObjectKey) (*corev1.Secret, error) {
	var secret corev1.Secret

	bh := p.client.Bucket(p.bucket)
	_, err := bh.Attrs(ctx)
	if err != nil {
		return nil, err
	}

	// Try to retrieve existing backup
	oh := bh.Object(path(key))
	_, err = oh.Attrs(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	oreader, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	// Copy state file from GCS
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, oreader)
	if err != nil {
		return nil, err
	}

	// Unmarshal state file into secret obj
	if err := yaml.Unmarshal(buf.Bytes(), &secret); err != nil {
		return nil, err
	}

	if err := oreader.Close(); err != nil {
		return nil, err
	}

	return &secret, nil
}

func path(key client.ObjectKey) string {
	return fmt.Sprintf("%s.yaml", key)
}
