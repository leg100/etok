package handlers

import (
	"errors"
	"fmt"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

var (
	ErrRunFailed = errors.New("run failed")
)

// RunConnectable returns true if the run indicates its container can be
// connected to. If isTTY is true then the container must be running and must
// not have completed. If isTTY is false then the container can either be
// running or have completed (because its logs can be retrieved).
func RunConnectable(name string, isTTY bool) watchtools.ConditionFunc {
	// Keep the last reason received for a run's 'complete' condition
	var lastReason string

	return func(event watch.Event) (bool, error) {
		run := event.Object.(*v1alpha1.Run)

		// ListWatcher field selector filters out other pods but the fake client
		// doesn't implement the field selector, so the following is necessary
		// purely for testing purposes
		if run.Name != name {
			return false, nil
		}

		switch event.Type {
		case watch.Deleted:
			return false, ErrResourceUnexpectedlyDeleted
		}

		if run.Conditions == nil {
			return false, nil
		}

		for _, condition := range run.Conditions {
			if condition.Type == v1alpha1.RunFailedCondition && condition.Status == metav1.ConditionTrue {
				return false, fmt.Errorf("%w: %s", ErrRunFailed, condition.Message)
			}

			if condition.Type == v1alpha1.RunCompleteCondition {
				if condition.Reason != lastReason {
					klog.V(1).Infof("run status update: %s: %s", condition.Reason, condition.Message)
				}
				lastReason = condition.Reason

				switch condition.Reason {
				case v1alpha1.PodRunningReason:
					return true, nil
				case v1alpha1.PodSucceededReason, v1alpha1.PodFailedReason:
					if isTTY {
						// We cannot attach to the TTY of a completed pod
						return false, PrematurelySucceededPodError
					}
					return true, nil
				default:
					return false, nil
				}
			}
		}
		return false, nil
	}
}
