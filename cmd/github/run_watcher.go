package github

import (
	"context"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/k8s/etokclient"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/util/slice"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// Watch webhook triggered runs, stream their logs and update their checkruns.
type runWatcher struct {
	// List of runs currently having their logs streamed
	streaming []string
	// k8s client
	client.Client
	// Function with which to retrieve logs from run pod
	logsFunc logstreamer.GetLogsFunc
	// Github client for updating check runs
	queue taskQueue
	// Frequency with which to update checkrun whilst a run's logs are being
	// streamed
	interval time.Duration
}

func (rw *runWatcher) watch(ctx context.Context) error {
	lw := &runListWatcher{Client: rw.EtokClient}
	_, err := watchtools.UntilWithSync(ctx, lw, &v1alpha1.Run{}, nil, func(event watch.Event) (bool, error) {
		obj := event.Object.(*v1alpha1.Run)

		lbls := obj.GetLabels()
		if lbls == nil {
			return false, nil
		}

		// Ignore runs that aren't labelled with a check suite ID
		if _, ok := lbls[checkSuiteLabelName]; !ok {
			return false, nil
		}

		// Ignore completed runs
		if val, ok := lbls[checkRunStatusLabelName]; ok && val == "completed" {
			return false, nil
		}

		// Construct a github run from resource
		run, err := newRunFromResource(obj)
		if err != nil {
			return false, err
		}

		// Label failed runs and update check run one last time
		if meta.IsStatusConditionTrue(obj.Conditions, v1alpha1.RunFailedCondition) {
			if err = setRunLabel(rw.Client, obj, checkRunStatusLabelName, "completed"); err != nil {
				return false, err
			}

			run.completed = true

			rw.queue.send(run)

			return false, nil
		}

		// Ok, so we have an incomplete run that was triggered by a check run
		// event. We want to stream its logs, sending regular updates to GH
		// through to its completion. We first need to check if we're already
		// doing this.
		if slice.ContainsString(rw.streaming, klog.KObj(obj).String()) {
			// Already being monitored
			return false, nil
		}

		// Add run to the list of runs being streamed
		rw.streaming = append(rw.streaming, klog.KObj(obj).String())
		go func() {
			// Stream run logs and send regular checkrun updates
			m := monitored{
				run:      run,
				Client:   rw.Client,
				logsFunc: rw.logsFunc,
			}
			monitor(rw.queue, &m, rw.interval)

			slice.DeleteString(rw.streaming, klog.KObj(obj).String())
		}()

		return false, nil
	})

	return err
}

// Monitored is a wrapper around a run that satisfies the process interface
type monitored struct {
	*run
	client.Client
	logsFunc logstreamer.GetLogsFunc
}

func (m *monitored) start() error {
	return logstreamer.Stream(context.Background(), m.logsFunc, m, m.PodsClient(m.namespace), m.id, globals.RunnerContainerName)
}

type runListWatcher struct {
	Client etokclient.Interface
}

func (lw *runListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	return lw.Client.EtokV1alpha1().Runs("").List(context.Background(), options)
}

func (lw *runListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return lw.Client.EtokV1alpha1().Runs("").Watch(context.Background(), options)
}

// Set a label on a Run and update the resource. Backs off and retry upon
// conflict.
func setRunLabel(client client.Client, run *v1alpha1.Run, name, value string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		run, err := client.RunsClient(run.Namespace).Get(context.Background(), run.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		lbls := run.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}

		lbls[name] = value
		run.SetLabels(lbls)

		_, err = client.RunsClient(run.Namespace).Update(context.Background(), run, metav1.UpdateOptions{})
		return err
	})
}
