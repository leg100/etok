package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// isAlreadyOwner checks if object already references owner in its
// ownerReferences
func isAlreadyOwner(owner, object controllerutil.Object, scheme *runtime.Scheme) bool {
	owners := object.GetOwnerReferences()
	ref := newOwnerReference(owner, object, scheme)
	idx := indexOwnerRef(owners, ref)
	if idx == -1 {
		return false
	}
	return true
}

// indexOwnerRef returns the index of the owner reference in the slice if found,
// or -1.
func indexOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) int {
	for index, r := range ownerReferences {
		if referSameObject(r, ref) {
			return index
		}
	}
	return -1
}

func newOwnerReference(owner, object controllerutil.Object, scheme *runtime.Scheme) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: object.GetObjectKind().GroupVersionKind().Version,
		Kind:       object.GetObjectKind().GroupVersionKind().Kind,
		UID:        owner.GetUID(),
		Name:       owner.GetName(),
	}
}

// Returns true if a and b point to the same object
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name && a.UID == b.UID
}
