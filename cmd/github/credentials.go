package github

import (
	"context"
	"strconv"
	"time"

	"github.com/leg100/etok/pkg/vcs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type credentials struct {
	client    kubernetes.Interface
	namespace string
	name      string

	// Timeout for waiting for credentials secret to be created
	timeout time.Duration
}

func (c *credentials) exists(ctx context.Context) (bool, error) {
	_, err := c.client.CoreV1().Secrets(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
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

func (c *credentials) create(ctx context.Context, s *vcs.GithubAppTemporarySecrets) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: c.name,
		},
		StringData: map[string]string{
			"id":             strconv.FormatInt(s.ID, 10),
			"key":            s.Key,
			"webhook-secret": s.WebhookSecret,
		},
	}

	_, err := c.client.CoreV1().Secrets(c.namespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

func (c *credentials) String() string {
	return c.namespace + "/" + c.name
}
