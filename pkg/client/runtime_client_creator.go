package client

import (
	"fmt"

	"github.com/leg100/etok/pkg/scheme"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// RuntimeClientCreator impls are objs for deferred creation of kubernetes clients
type RuntimeClientCreator interface {
	CreateRuntimeClient(string) (*Client, error)
}

// Implements RuntimeClientCreator
type runtimeClientCreator struct{}

func NewRuntimeClientCreator() RuntimeClientCreator {
	return &runtimeClientCreator{}
}

func (cc *runtimeClientCreator) CreateRuntimeClient(kubeCtx string) (*Client, error) {
	cfg, err := config.GetConfigWithContext(kubeCtx)
	if err != nil {
		return nil, fmt.Errorf("getting kubernetes client config: %w", err)
	}

	rc, err := runtimeclient.New(cfg, runtimeclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, fmt.Errorf("creating controller-runtime client: %w", err)
	}

	return &Client{
		Config:        cfg,
		RuntimeClient: rc,
	}, nil
}
