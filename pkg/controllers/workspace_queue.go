package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/launcher"
	"github.com/leg100/etok/pkg/util/slice"
	"k8s.io/apimachinery/pkg/api/meta"
)

func updateQueue(ws *v1alpha1.Workspace, runs []v1alpha1.Run) (queue []string) {
	// Filter run resources
	for _, run := range runs {
		// Filter out runs belonging to other workspaces
		if run.Workspace != ws.Name {
			continue
		}

		// Filter out completed runs
		if meta.IsStatusConditionTrue(run.Conditions, v1alpha1.DoneCondition) {
			continue
		}

		// Filter out non-queueable runs
		if !launcher.IsQueueable(run.Command) {
			continue
		}

		// Filter out privileged commands that are yet to be approved
		if slice.ContainsString(ws.Spec.PrivilegedCommands, run.Command) {
			if !ws.IsRunApproved(&run) {
				continue
			}
		}

		queue = append(queue, run.Name)
	}

	// Re-order new queue to ensure runs maintain their position from the
	// existing queue
	for i := len(ws.Status.Queue) - 1; i >= 0; i-- {
		j := slice.StringIndex(queue, ws.Status.Queue[i])
		if j == -1 {
			// Skip run not found in new queue
			continue
		}
		if j == 0 {
			// No need to move run
			continue
		}
		// Move run to front of queue
		queue = append(queue[:j], queue[j+1:]...)
		queue = append([]string{ws.Status.Queue[i]}, queue...)
	}

	return queue
}
