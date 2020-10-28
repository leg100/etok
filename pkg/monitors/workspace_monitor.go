package monitors

import (
	"context"
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
)

// The workspaceMonitor object has various handlers for monitoring a workspace's pod
type workspaceMonitor struct {
	ws     *v1alpha1.Workspace
	client stokclient.Interface
}

func (mon *workspaceMonitor) monitor(ctx context.Context, errch chan<- error) {
	lw := &k8s.WorkspaceListWatcher{Client: mon.client, Name: mon.ws.GetName(), Namespace: mon.ws.GetNamespace()}

	go func() {
		_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Workspace{}, nil, mon.workspaceHealthyHandler)
		if err != nil {
			errch <- err
		}
	}()
}

// Return true if pod is both running and ready
func (mon *workspaceMonitor) workspaceHealthyHandler(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("workspace resource deleted")
	}

	switch ws := event.Object.(type) {
	case *v1alpha1.Workspace:
		if ws.Status.Conditions == nil {
			return false, nil
		}
		if ws.Status.Conditions.IsTrueFor(v1alpha1.ConditionHealthy) {
			return true, nil
		}
	}
	return false, nil
}
