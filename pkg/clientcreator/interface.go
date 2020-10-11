package clientcreator

import (
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
)

type Interface interface {
	KubeClient() kubernetes.Interface
	StokClient() stokclient.Interface
	CreateClients(string) error
}
