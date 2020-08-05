package k8s

import (
	"context"
	"fmt"
	"time"

	watchtools "k8s.io/client-go/tools/watch"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubectl/pkg/util/interrupt"
)

// (deprecated) Wrapper for watchtools.UntilWithSync
func WaitUntil(rc rest.Interface, obj runtime.Object, name, namespace, plural string, exitCondition watchtools.ConditionFunc, timeout time.Duration) (runtime.Object, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", name)
	lw := cache.NewListWatchFromClient(rc, plural, namespace, fieldSelector)

	ctx, cancel := watchtools.ContextWithOptionalTimeout(context.Background(), timeout)
	defer cancel()

	intr := interrupt.New(nil, cancel)

	var result runtime.Object
	err := intr.Run(func() error {
		ev, err := watchtools.UntilWithSync(ctx, lw, obj, nil, exitCondition)
		if ev != nil {
			result = ev.Object
		}
		return err
	})
	return result, err
}

func GetNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

// PodRunningAndReady returns true if the pod is running and ready, false if the pod has not
// yet reached those states, returns error in any other case
func PodRunningAndReady(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "")
	}

	switch t := event.Object.(type) {
	case *corev1.Pod:
		switch t.Status.Phase {
		case corev1.PodSucceeded:
			return false, fmt.Errorf("pod finished")
		case corev1.PodFailed:
			return false, fmt.Errorf(t.Status.ContainerStatuses[0].State.Terminated.Message)
		case corev1.PodRunning:
			conditions := t.Status.Conditions
			if conditions == nil {
				return false, nil
			}
			for i := range conditions {
				if conditions[i].Type == corev1.PodReady &&
					conditions[i].Status == corev1.ConditionTrue {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
