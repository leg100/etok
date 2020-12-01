package scheme

import (
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	// Create new scheme obj to add built-in and etok schemes to below
	Scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(v1alpha1.AddToScheme(Scheme))
}
