package backup

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeProvider struct {
	BucketObjs []*corev1.Secret
}

func (p *FakeProvider) Backup(ctx context.Context, secret *corev1.Secret) error {
	p.BucketObjs = append(p.BucketObjs, secret)

	return nil
}

func (p *FakeProvider) Restore(ctx context.Context, key client.ObjectKey) (*corev1.Secret, error) {
	for _, obj := range p.BucketObjs {
		if client.ObjectKeyFromObject(obj) == key {
			return obj, nil
		}
	}

	return nil, nil
}
