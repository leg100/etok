package controllers

import (
	"context"
	"fmt"
	"reflect"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/util/slice"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// updateQueue queries Runs and updates the Workspace queue accordingly,
// including the active run.
func updateQueue(c client.Client, ws *v1alpha1.Workspace) error {
	// New queue to be built and compared to old (existing) queue
	newQ := []string{}
	oldQ := append([]string{ws.Status.Active}, ws.Status.Queue...)

	// Fetch run resources
	runlist := &v1alpha1.RunList{}
	if err := c.List(context.TODO(), runlist, client.InNamespace(ws.Namespace)); err != nil {
		return err
	}

	// Filter run resources
	meta.EachListItem(runlist, func(o runtime.Object) error {
		run := o.(*v1alpha1.Run)

		// Filter out commands belonging to other workspaces
		if run.GetWorkspace() != ws.Name {
			return nil
		}

		// Filter out completed commands
		if run.GetPhase() == v1alpha1.RunPhaseCompleted {
			return nil
		}

		// Filter out privileged commands that are yet to be approved
		if slice.ContainsString(ws.Spec.PrivilegedCommands, run.Command) {
			if !ws.IsRunApproved(run) {
				return nil
			}
		}

		newQ = append(newQ, run.GetName())
		return nil
	})

	// Re-order new queue to ensure runs maintain their position from the old
	// queue
	for i := len(oldQ) - 1; i >= 0; i-- {
		j := slice.StringIndex(newQ, oldQ[i])
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
		newQ = append([]string{oldQ[i]}, newQ...)
	}

	// Update status if queue has changed
	if !reflect.DeepEqual(newQ, oldQ) {
		switch len(newQ) {
		case 0:
			ws.Status.Active = ""
			ws.Status.Queue = []string{}
		default:
			ws.Status.Active = newQ[0]
			ws.Status.Queue = newQ[1:]
		}
		if err := c.Status().Update(context.TODO(), ws); err != nil {
			return fmt.Errorf("Failed to update queue: %w", err)
		}
	}
	return nil
}
