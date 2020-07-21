package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// Interface combining runtime.Object and metav1.Object. Anticipates:
// 	https://github.com/kubernetes-sigs/controller-runtime/pull/898
type Object interface {
	runtime.Object
	metav1.Object
}
