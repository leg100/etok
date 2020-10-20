package k8s

import (
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

// Delete annotation WaitAnnotationKey, giving the runner the signal to start
func DeleteWaitAnnotationKey(obj metav1.Object) {
	annotations := obj.GetAnnotations()
	delete(annotations, v1alpha1.WaitAnnotationKey)
	obj.SetAnnotations(annotations)
}

