package github

import (
	"context"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/k8s/etokclient"
	"github.com/leg100/etok/pkg/util/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/klog/v2"
)

const (
	checkSuiteLabelName      = "etok.dev/github-checksuite-id"
	checkRunStatusLabelName  = "etok.dev/github-checkrun-status"
	checkRunCommandLabelName = "etok.dev/github-checkrun-command"
	checkRunSHALabelName     = "etok.dev/github-checkrun-sha"
)

type runMonitor struct {
	monitored []string
}

func newRunMonitor() *runMonitor {
	return &runMonitor{}
}

func (m *runMonitor) watch(ctx context.Context, client etokclient.Interface) error {
	lw := &RunListWatcher{Client: client}
	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Run{}, nil, func(event watch.Event) (bool, error) {
		run := event.Object.(*v1alpha1.Run)

		// Ignore runs that aren't labelled with a check suite ID
		lbls := run.GetLabels()
		if lbls == nil {
			return false, nil
		}
		if _, ok := lbls[checkSuiteLabelName]; !ok {
			return false, nil
		}

		// If a run has a checkrun that is completed then we no longer need to
		// monitor it
		if val, ok := lbls[checkRunStatusLabelName]; ok && val == "completed" {
			slice.DeleteString(m.monitored, klog.KObj(run).String())
			return false, nil
		}

		// Ok, so we have an incomplete run that was triggered by a check run
		// event. We want to stream its logs, sending regular updates to GH
		// through to its completion. We first need to check if we're already
		// doing this.
		if slice.ContainsString(m.monitored, klog.KObj(run).String()) {
			// Already being monitored
			return false, nil
		}

		return false, nil
	})

	return err
}

type RunListWatcher struct {
	Client etokclient.Interface
}

func (lw *RunListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	return lw.Client.EtokV1alpha1().Runs("").List(context.Background(), options)
}

func (lw *RunListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return lw.Client.EtokV1alpha1().Runs("").Watch(context.Background(), options)
}
