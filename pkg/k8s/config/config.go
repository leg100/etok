package config

import (
	"sync"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	kubeConfig     *rest.Config
	kubeConfigOnce sync.Once
)

// Get k8s config, caching on first call
func GetConfig() (*rest.Config, error) {
	var err error
	kubeConfigOnce.Do(func() {
		kubeConfig, err = config.GetConfigWithContext(kubeContext)
	})
	return kubeConfig, err
}
