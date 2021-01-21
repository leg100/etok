package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/launcher"
	"github.com/leg100/etok/pkg/util/slice"
)

// updateCombinedQueue updates a workspace's combined queue (the active run +
// the queue) with the given list of runs.  Runs in the existing queue are
// expunged if they meet certain criteria.  If they are not expunged they
// mantain their position.
func updateCombinedQueue(ws *v1alpha1.Workspace, runs []v1alpha1.Run) {
	newQ := []string{}
	currQ := append([]string{ws.Status.Active}, ws.Status.Queue...)

	// Filter run resources
	for _, run := range runs {
		// Filter out runs belonging to other workspaces
		if run.Workspace != ws.Name {
			continue
		}

		// Filter out completed runs
		if run.IsDone() {
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

		newQ = append(newQ, run.Name)
	}

	// Re-order new queue to ensure runs maintain their position from the
	// existing queue
	for i := len(currQ) - 1; i >= 0; i-- {
		j := slice.StringIndex(newQ, currQ[i])
		if j == -1 {
			// Skip run not found in new queue
			continue
		}
		if j == 0 {
			// No need to move run
			continue
		}
		// Move run to front of queue
		newQ = append(newQ[:j], newQ[j+1:]...)
		newQ = append([]string{currQ[i]}, newQ...)
	}

	// Update workspace with new (combined) queue
	if len(newQ) > 0 {
		ws.Status.Active, ws.Status.Queue = newQ[0], newQ[1:]
	} else {
		ws.Status.Active, ws.Status.Queue = "", []string(nil)
	}
}
