package handlers

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/util/slice"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
)

// WorkspaceInitializing returns a handler that watches a workspace until it
// reports an initializing or ready phase.
func WorkspaceInitializing(runName string) watchtools.ConditionFunc {
	return workspaceHandlerWrapper(func(ws *v1alpha1.Workspace) (bool, error) {
		switch ws.Status.Phase {
		case v1alpha1.WorkspacePhaseInitializing, v1alpha1.WorkspacePhaseReady:
			return true, nil
		case v1alpha1.WorkspacePhaseError
			boldCyan := color.New(color.FgCyan, color.Bold).SprintFunc()
			var printedQueue []string
			for _, run := range ws.Status.Queue {
				if run == runName {
					printedQueue = append(printedQueue, boldCyan(run))
				} else {
					printedQueue = append(printedQueue, run)
				}
			}
			fmt.Printf("Queued: %v\n", printedQueue)
		default:
			// yet to be queued
			return false, nil
		}
		return false, nil
	})
}

// Log queue position until run is at front of queue
func LogQueuePosition(runName string) watchtools.ConditionFunc {
	return workspaceHandlerWrapper(func(ws *v1alpha1.Workspace) (bool, error) {
		pos := slice.StringIndex(ws.Status.Queue, runName)
		switch {
		case pos == 0:
			return true, nil
		case pos > 0:
			boldCyan := color.New(color.FgCyan, color.Bold).SprintFunc()
			var printedQueue []string
			for _, run := range ws.Status.Queue {
				if run == runName {
					printedQueue = append(printedQueue, boldCyan(run))
				} else {
					printedQueue = append(printedQueue, run)
				}
			}
			fmt.Printf("Queued: %v\n", printedQueue)
		default:
			// yet to be queued
			return false, nil
		}
		return false, nil
	})
}

// Return true if run is queued
func IsQueued(runName string) watchtools.ConditionFunc {
	return workspaceHandlerWrapper(func(ws *v1alpha1.Workspace) (bool, error) {
		if slice.ContainsString(ws.Status.Queue, runName) {
			return true, nil
		}
		return false, nil
	})
}

type workspaceHandler func(*v1alpha1.Workspace) (bool, error)

// Event handler wrapper for workspace object events
func workspaceHandlerWrapper(handler workspaceHandler) watchtools.ConditionFunc {
	return func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Deleted:
			return false, ErrResourceUnexpectedlyDeleted
		}

		switch ws := event.Object.(type) {
		case *v1alpha1.Workspace:
			return handler(ws)
		}

		return false, nil
	}
}
