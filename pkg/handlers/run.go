package handlers

import (
	"fmt"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

// Log run's phase
func LogRunPhase() watchtools.ConditionFunc {
	// Current run phase
	var phase v1alpha1.RunPhase
	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, fmt.Errorf("run resource deleted")
		}

		switch run := event.Object.(type) {
		case *v1alpha1.Run:
			if run.GetPhase() != phase {
				klog.V(1).Infof("new run phase: %s\n", run.GetPhase())
				phase = run.GetPhase()
			}
		}
		return false, nil
	}
}
