package launcher

import (
	"context"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type Launcher struct {
	Name           string
	Workspace      string
	Namespace      string
	Command        string
	Path           string
	Args           []string
	TimeoutClient  time.Duration
	TimeoutPod     time.Duration
	TimeoutQueue   time.Duration
	TimeoutEnqueue time.Duration

	Debug bool
}

func (t *Launcher) Run(ctx context.Context) error {
	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	kc, err := k8s.KubeClient()
	if err != nil {
		return err
	}

	// Tar up local config and deploy k8s resources
	run, err := t.deploy(ctx, sc, kc)
	if err != nil {
		return err
	}

	// Monitor resources, wait until pod is running and ready
	if err := t.monitor(ctx, sc, kc, run); err != nil {
		return err
	}

	// Attach to pod, and release hold annotation
	return k8s.PodConnect(ctx, kc, t.Namespace, t.Name, func() error {
		runsclient := sc.StokV1alpha1().Runs(t.Namespace)
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			run, err := runsclient.Get(ctx, t.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			k8s.DeleteWaitAnnotationKey(run)

			_, err = runsclient.Update(ctx, run, metav1.UpdateOptions{})
			return err
		})
	})
}

func (t *Launcher) monitor(ctx context.Context, sc stokclient.Interface, kc kubernetes.Interface, run *v1alpha1.Run) error {
	errch := make(chan error)
	ready := make(chan struct{})

	// Non-blocking; watch workspace queue, check timeouts are not exceeded, and log run's queue position
	(&queueMonitor{
		run:            run,
		workspace:      t.Workspace,
		client:         sc,
		timeoutEnqueue: t.TimeoutEnqueue,
		timeoutQueue:   t.TimeoutQueue,
	}).monitor(ctx, errch)

	// Non-blocking; watch run, log status updates
	(&runMonitor{
		run:    run,
		client: sc,
	}).monitor(ctx, errch)

	// Non-blocking; watch run's pod, sends to ready when pod is running and ready to attach to, or
	// error on fatal pod errors
	(&podMonitor{
		run:    run,
		client: kc,
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
func (t *Launcher) deploy(ctx context.Context, sc stokclient.Interface, kc kubernetes.Interface) (run *v1alpha1.Run, err error) {
	g, ctx := errgroup.WithContext(ctx)

	// Compile tarball of terraform module, embed in configmap and deploy
	g.Go(func() error {
		tarball, err := archive.Create(t.Path)
		if err != nil {
			return err
		}

		// Construct and deploy ConfigMap resource
		return t.createConfigMap(ctx, kc, tarball, t.Name, v1alpha1.RunDefaultConfigMapKey)
	})

	// Construct and deploy command resource
	g.Go(func() error {
		run, err = t.createRun(ctx, sc, t.Name, t.Name)
		return err
	})

	return run, g.Wait()
}
