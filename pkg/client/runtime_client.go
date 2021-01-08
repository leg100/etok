package client

import (
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// RuntimeClient is a collection of kubernetes clients along with some convenience methods.
type RuntimeClient struct {
	// K8s client config
	Config *rest.Config

	// Controller-runtime client
	runtimeclient.Client
}
