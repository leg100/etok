package client

import (
	"context"
	"fmt"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"

	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Client is a Kubernetes client factory, along with methods for interacting with pods.
type Client interface {
	Create(string) error
	KubeConfig() *rest.Config
	KubeClient() kubernetes.Interface
	StokClient() stokclient.Interface
	GetLogs(context.Context, *corev1.Pod, string) (io.ReadCloser, error)
	Attach(*corev1.Pod, string, string, *os.File, io.Writer) error
}

// Implements Client
type client struct {
	// Client config
	config *rest.Config

	// Kubernetes built-in client
	kubeClient kubernetes.Interface

	// Stok generated client
	stokClient stokclient.Interface
}

func NewClient() Client {
	return &client{}
}

func (c *client) KubeConfig() *rest.Config {
	return c.config
}

func (c *client) KubeClient() kubernetes.Interface {
	return c.kubeClient
}

func (c *client) StokClient() stokclient.Interface {
	return c.stokClient
}

func (c *client) Create(kubeCtx string) error {
	cfg, err := config.GetConfigWithContext(kubeCtx)
	if err != nil {
		return fmt.Errorf("getting kubernetes client config: %w", err)
	}

	sc, err := stokclient.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating stok kubernetes client: %w", err)
	}

	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating built-in kubernetes client: %w", err)
	}

	c.config = cfg
	c.stokClient = sc
	c.kubeClient = kc

	return nil
}

func (c *client) GetLogs(ctx context.Context, pod *corev1.Pod, container string) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{Follow: true, Container: container}
	return c.KubeClient().CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), opts).Stream(ctx)
}
