package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Interface combining runtime.Object and metav1.Object. Anticipates:
// 	https://github.com/kubernetes-sigs/controller-runtime/pull/898
type Object interface {
	runtime.Object
	metav1.Object
}

func NewObjectFromGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) (Object, error) {
	obj, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	return obj.(Object), nil
}
