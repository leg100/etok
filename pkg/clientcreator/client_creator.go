package clientcreator

import (
	"fmt"

	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Interface interface {
	KubeConfig() *rest.Config
	KubeClient() kubernetes.Interface
	StokClient() stokclient.Interface
	CreateClients(string) error
}

// Implements Interface
type ClientCreator struct {
	// Client config
	config *rest.Config

	// Kubernetes built-in client
	kubeClient kubernetes.Interface

	// Stok generated client
	stokClient stokclient.Interface
}

func NewClientCreator() *ClientCreator {
	return &ClientCreator{}
}

func (cc *ClientCreator) KubeConfig() *rest.Config {
	return cc.config
}

func (cc *ClientCreator) KubeClient() kubernetes.Interface {
	return cc.kubeClient
}

func (cc *ClientCreator) StokClient() stokclient.Interface {
	return cc.stokClient
}

func (cc *ClientCreator) CreateClients(kubeCtx string) error {
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

	cc.config = cfg
	cc.stokClient = sc
	cc.kubeClient = kc

	return nil
}
