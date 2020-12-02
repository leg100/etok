package client

import (
	"github.com/leg100/etok/pkg/k8s/etokclient"
	etoktyped "github.com/leg100/etok/pkg/k8s/etokclient/typed/etok.dev/v1alpha1"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// Client is a collection of kubernetes clients along with some convenience methods.
type Client struct {
	// Client config
	Config *rest.Config

	// Kubernetes built-in client
	KubeClient kubernetes.Interface

	// etok generated client
	EtokClient etokclient.Interface
}

func (c *Client) PodsClient(namespace string) typedv1.PodInterface {
	return c.KubeClient.CoreV1().Pods(namespace)
}

func (c *Client) ServiceAccountsClient(namespace string) typedv1.ServiceAccountInterface {
	return c.KubeClient.CoreV1().ServiceAccounts(namespace)
}

func (c *Client) SecretsClient(namespace string) typedv1.SecretInterface {
	return c.KubeClient.CoreV1().Secrets(namespace)
}

func (c *Client) ConfigMapsClient(namespace string) typedv1.ConfigMapInterface {
	return c.KubeClient.CoreV1().ConfigMaps(namespace)
}

func (c *Client) WorkspacesClient(namespace string) etoktyped.WorkspaceInterface {
	return c.EtokClient.EtokV1alpha1().Workspaces(namespace)
}

func (c *Client) RunsClient(namespace string) etoktyped.RunInterface {
	return c.EtokClient.EtokV1alpha1().Runs(namespace)
}
