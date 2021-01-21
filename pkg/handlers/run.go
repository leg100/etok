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

// Event handler for a run that returns true once its status shows its pod is
// running. If the status shows the run has failed an error is returned
// containing the reason for why it failed.
func RunPodRunning(name string) watchtools.ConditionFunc {
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
				if condition.Status == metav1.ConditionFalse {
					if condition.Reason != lastReason {
						klog.V(1).Infof("run status update: %s: %s", condition.Reason, condition.Message)
					}

					if condition.Reason == v1alpha1.PodRunningReason {
						return true, nil
					}

					lastReason = condition.Reason
				}
			}
		}
		return false, nil
	}
}
