package clientcreator

import (
	"fmt"

	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ClientCreator struct {
	// Kubernetes built-in client
	kubeClient kubernetes.Interface

	// Stok generated client
	stokClient stokclient.Interface
}

func  NewClientCreator() *ClientCreator {
	return &ClientCreator{}
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

	cc.stokClient= sc
	cc.kubeClient= kc

	return nil
}
