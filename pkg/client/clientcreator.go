package client

import (
	"fmt"

	"github.com/leg100/etok/pkg/k8s/etokclient"
	"github.com/leg100/etok/pkg/scheme"
	"k8s.io/client-go/kubernetes"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// ClientCreator impls are objs for deferred creation of kubernetes clients
type ClientCreator interface {
	Create(string) (*Client, error)
}

// Implements ClientCreator
type clientCreator struct{}

func NewClientCreator() ClientCreator {
	return &clientCreator{}
}

func (cc *clientCreator) Create(kubeCtx string) (*Client, error) {
	cfg, err := config.GetConfigWithContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes client config: %w", err)
	}

	sc, err := etokclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating etok kubernetes client: %w", err)
	}

	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating built-in kubernetes client: %w", err)
	}

	rc, err := runtimeclient.New(cfg, runtimeclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("creating controller-runtime client: %w", err)
	}

	return &Client{
		Config:        cfg,
		EtokClient:    sc,
		KubeClient:    kc,
		RuntimeClient: rc,
	}, nil
}
