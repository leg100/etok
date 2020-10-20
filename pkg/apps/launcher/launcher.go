package launcher

import (
	"context"
	"fmt"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/app"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/podhandler"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type Launcher struct {
	*app.Options
	PodHandler podhandler.Interface
}

func NewFromOpts(opts *app.Options) app.App {
	return &Launcher{
		Options:    opts,
		PodHandler: &podhandler.PodHandler{},
	}
}

func (t *Launcher) Run(ctx context.Context) error {
	// Tar up local config and deploy k8s resources
	run, err := t.deploy(ctx)
	if err != nil {
		return err
	}

	// Monitor resources, wait until pod is running and ready
	if err := t.monitor(ctx, run); err != nil {
		return err
	}

	pod, err := t.KubeClient().CoreV1().Pods(t.Namespace).Get(ctx, t.RunName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting pod %s/%s: %w", t.Namespace, t.RunName, err)
	}

	// Attach to pod, and release hold annotation
	return k8s.PodConnect(ctx, t.PodHandler, t.KubeClient(), t.KubeConfig(), pod, t.Out, func() error {
		runsclient := t.StokClient().StokV1alpha1().Runs(t.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			run, err := runsclient.Get(ctx, t.RunName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			k8s.DeleteWaitAnnotationKey(run)

			_, err = runsclient.Update(ctx, run, metav1.UpdateOptions{})
			return err
		})
	})
}

func (t *Launcher) monitor(ctx context.Context, run *v1alpha1.Run) error {
	errch := make(chan error)
	ready := make(chan struct{})

	// Non-blocking; watch workspace queue, check timeouts are not exceeded, and log run's queue position
	(&queueMonitor{
		run:            run,
		workspace:      t.Workspace,
		client:         t.StokClient(),
		timeoutEnqueue: t.TimeoutEnqueue,
		timeoutQueue:   t.TimeoutQueue,
	}).monitor(ctx, errch)

	// Non-blocking; watch run, log status updates
	(&runMonitor{
		run:    run,
		client: t.StokClient(),
	}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		run:    run,
		client: t.KubeClient(),
	}).monitor(ctx, errch, ready)

	// Wait for pod to be ready
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errch:
		return err
	}
}

// Deploy configmap and run resources in parallel
func (t *Launcher) deploy(ctx context.Context) (run *v1alpha1.Run, err error) {
	g, ctx := errgroup.WithContext(ctx)

	// Compile tarball of terraform module, embed in configmap and deploy
	g.Go(func() error {
		tarball, err := archive.Create(t.Path)
		if err != nil {
			return err
		}

		// Construct and deploy ConfigMap resource
		return t.createConfigMap(ctx, tarball, t.RunName, v1alpha1.RunDefaultConfigMapKey)
	})

	// Construct and deploy command resource
	g.Go(func() error {
		run, err = t.createRun(ctx, t.RunName, t.RunName)
		return err
	})

	return run, g.Wait()
}
