package dependents

import (
	"k8s.io/apimachinery/pkg/runtime"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Dependent interface {
	metav1.Object
	runtime.Object
	GetRuntimeObj() runtime.Object
	Construct() error
}
