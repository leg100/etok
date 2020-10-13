package clientcreator

import (
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Interface interface {
	KubeConfig() *rest.Config
	KubeClient() kubernetes.Interface
	StokClient() stokclient.Interface
	CreateClients(string) error
}
