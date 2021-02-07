package backup

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider interface {
	Backup(context.Context, *corev1.Secret) error
	Restore(context.Context, client.ObjectKey) (*corev1.Secret, error)
}
