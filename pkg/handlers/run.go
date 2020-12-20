package handlers

import (
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

// Log run's phase
func LogRunPhase() watchtools.ConditionFunc {
	// Current run phase
	var phase = v1alpha1.RunPhaseUnknown

	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, ErrResourceUnexpectedlyDeleted
		}

		switch run := event.Object.(type) {
		case *v1alpha1.Run:
			if run.Phase != phase {
				klog.V(1).Infof("run phase shift: %s -> %s\n", phase, run.Phase)
				phase = run.Phase
			}
		}
		return false, nil
	}
}
