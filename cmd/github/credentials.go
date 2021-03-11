package github

import (
	"context"
	"strconv"
	"time"

	"github.com/google/go-github/v31/github"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// name of the secret containing the github app credentials
	secretName = "creds"
)

type credentials struct {
	client    runtimeclient.Client
	namespace string

	// Timeout for waiting for credentials secret to be created
	timeout time.Duration
}

func (c *credentials) exists(ctx context.Context) (bool, error) {
	secret := corev1.Secret{}
	err := c.client.Get(ctx, types.NamespacedName{Namespace: c.namespace, Name: secretName}, &secret)
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Wait for secret to be created
func (c *credentials) poll(ctx context.Context) error {
	return wait.PollImmediate(time.Second, c.timeout, func() (bool, error) {
		klog.V(2).Infof("polling for secret: %s", c)
		return c.exists(ctx)
	})
}

func (c *credentials) create(ctx context.Context, cfg *github.AppConfig) error {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: c.namespace,
			Name:      secretName,
		},
		StringData: map[string]string{
			"id":             strconv.FormatInt(cfg.GetID(), 10),
			"key":            cfg.GetPEM(),
			"webhook-secret": cfg.GetWebhookSecret(),
		},
	}

	return c.client.Create(ctx, &secret)
}

func (c *credentials) String() string {
	return c.namespace + "/" + secretName
}
