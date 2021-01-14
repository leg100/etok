package handlers

import (
	"errors"
	"fmt"
	"io"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchtools "k8s.io/client-go/tools/watch"
)

var (
	ErrRestoreFailed = errors.New("restore failed")
)

// Handler that waits for the workspace restore failure condition status to be
// either true or false. If true, return the failure message in an error. If
// false, print out the message (which'll indicate whether there was anything
// restored).
func Restore(out io.Writer) watchtools.ConditionFunc {
	return workspaceHandlerWrapper(func(ws *v1alpha1.Workspace) (bool, error) {
		cond := meta.FindStatusCondition(ws.Status.Conditions, v1alpha1.RestoreFailureCondition)
		if cond == nil {
			return false, nil
		}
		switch cond.Status {
		case metav1.ConditionFalse:
			fmt.Fprintf(out, "Restore status: %s\n", cond.Message)
			return true, nil
		case metav1.ConditionTrue:
			return false, fmt.Errorf("%w: %s", ErrRestoreFailed, cond.Message)
		}
		return false, nil
	})
}
