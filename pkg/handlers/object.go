package handlers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
)

// reconcilable is a kubernetes resource that can state whether it has been
// reconciled or not.
type reconcilable interface {
	metav1.Object
	// IsReconciled is true if obj has been reconciled at least once by an
	// operator.
	IsReconciled() bool
}

// Handler that returns true when a resource has been reconciled. Reconciled
// here means it has been reconciled at least once. This handler is useful to
// determine if an operator is functioning.
func Reconciled(obj reconcilable) watchtools.ConditionFunc {
	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, ErrResourceUnexpectedlyDeleted
		}

		eventObj, ok := event.Object.(reconcilable)
		if !ok {
			// Skip non-reconcilable objects
			return false, nil
		}

		if eventObj.GetName() != obj.GetName() {
			return false, nil
		}

		if eventObj.IsReconciled() {
			// Success: resource has been reconciled
			return true, nil
		}

		// Obj is yet to be reconciled
		return false, nil
	}
}
