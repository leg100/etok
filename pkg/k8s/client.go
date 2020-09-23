package k8s

import (
	"fmt"

	"github.com/leg100/stok/pkg/k8s/config"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
)

// For testing purposes
var (
	KubeClient = getKubeClient
	StokClient = getStokClient
)

func getKubeClient() (kubernetes.Interface, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("getting client config for built-in kubernetes client: %w", err)
	}
	return kubernetes.NewForConfig(cfg)
}

func getStokClient() (stokclient.Interface, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("getting client config for stok kubernetes client: %w", err)
	}
	return stokclient.NewForConfig(cfg)
}
