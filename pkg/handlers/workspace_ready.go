package handlers

import (
	"errors"
	"fmt"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchtools "k8s.io/client-go/tools/watch"
)

var (
	ErrWorkspaceFailed   = errors.New("workspace is in a failure state")
	ErrWorkspaceDeletion = errors.New("workspace is currently being deleted")
)

// WorkspaceReady returns true when a workspace's ready condition is true. An
// error is returned if the ready condition is false with a failure reason or it
// a deletion reason (its undergoing deletion).  Any other reason returns false.
func WorkspaceReady() watchtools.ConditionFunc {
	return workspaceHandlerWrapper(func(ws *v1alpha1.Workspace) (bool, error) {
		cond := meta.FindStatusCondition(ws.Status.Conditions, v1alpha1.WorkspaceReadyCondition)
		if cond == nil {
			return false, nil
		}
		switch cond.Status {
		case metav1.ConditionTrue:
			return true, nil
		case metav1.ConditionFalse:
			switch cond.Reason {
			case v1alpha1.FailureReason:
				return false, fmt.Errorf("%w: %s", ErrWorkspaceFailed, cond.Message)
			case v1alpha1.DeletionReason:
				return false, ErrWorkspaceDeletion
			}
		}
		return false, nil
	})
}
