package client

import (
	"testing"

	"github.com/leg100/etok/pkg/k8s/etokclient"
	"github.com/leg100/etok/pkg/scheme"
	"k8s.io/client-go/kubernetes"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func BenchmarkClientCreate(b *testing.B) {
	cfg, _ := config.GetConfigWithContext("")

	b.Run("runtime", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			_, _ = runtimeclient.New(cfg, runtimeclient.Options{Scheme: scheme.Scheme})
		}
	})

	b.Run("kube", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			_, _ = kubernetes.NewForConfig(cfg)
		}
	})

	b.Run("etok", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			_, _ = etokclient.NewForConfig(cfg)
		}
	})
}
