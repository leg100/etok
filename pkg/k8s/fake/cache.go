package fake

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	runtimecache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
)

type Cache struct {
	runtimecache.Cache
}

var _ runtimecache.Cache = &Cache{}

func NewCache(scheme *runtime.Scheme) *Cache {
	return &Cache{
		Cache: &informertest.FakeInformers{Scheme: scheme},
	}
}

func (c *Cache) Start(stop <-chan struct{}) error {
	<-stop
	return fmt.Errorf("cache stopped")
}
