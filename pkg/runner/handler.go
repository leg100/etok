package runner

import (
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func isSyncHandler(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("resource deleted")
	}

	switch t := event.Object.(type) {
	case metav1.Object:
		if _, ok := t.GetAnnotations()[v1alpha1.WaitAnnotationKey]; !ok {
			return true, nil
		}
	}

	return false, nil
}
