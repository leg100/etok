package k8s

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api"
	"github.com/leg100/stok/api/v1alpha1"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

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
	toolscache "k8s.io/client-go/tools/cache"
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

func EventHandlers(events chan interface{}) toolscache.ResourceEventHandler {
	return toolscache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { events <- obj },
		UpdateFunc: func(_, obj interface{}) { events <- obj },
		DeleteFunc: func(obj interface{}) { events <- obj },
	}
}

func GetNamespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

func ReleaseHold(sc Client, obj api.Object) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
		if err := sc.Get(context.TODO(), key, obj); err != nil {
			return err
		}

		// Delete annotation WaitAnnotationKey, giving the runner the signal to start
		annotations := obj.GetAnnotations()
		delete(annotations, v1alpha1.WaitAnnotationKey)
		obj.SetAnnotations(annotations)

		return sc.Update(context.TODO(), obj, &runtimeclient.UpdateOptions{})
	})
}

// Attach to pod, falling back to streaming logs on error
func AttachFallbackToLogs(sc Client, pod *corev1.Pod, logstream io.ReadCloser) error {
	err := sc.Attach(pod)
	if err != nil {
		// TODO: use log fields
		log.Warn("Failed to attach to pod TTY; falling back to streaming logs")
		_, err = io.Copy(os.Stdout, logstream)
		return err
	}
	return nil
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
