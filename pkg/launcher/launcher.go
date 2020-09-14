package launcher

import (
	"context"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/reporters"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type Launcher struct {
	Workspace      string
	Namespace      string
	Context        string
	Command        string
	Path           string
	Args           []string
	TimeoutClient  time.Duration
	TimeoutPod     time.Duration
	TimeoutQueue   time.Duration
	TimeoutEnqueue time.Duration

	Debug   bool
	Factory k8s.FactoryInterface
}

func (t *Launcher) Run(ctx context.Context) error {
	config, err := t.Factory.NewConfig(t.Context)
	if err != nil {
		return err
	}

	rc, err := t.Factory.NewClient(config)
	if err != nil {
		return err
	}

	// Generate unique name shared by command and configmap resources (and command ctrl will spawn a
	// pod with this name, too)
	name := t.Factory.GenerateName()

	errch := make(chan error)

	// Delete resources upon program termination.
	var resources []runtime.Object
	defer func() {
		for _, r := range resources {
			rc.Delete(context.TODO(), r)
		}
	}()

	// Compile tarball of terraform module, embed in configmap and deploy
	var uploaded = make(chan struct{})
	go func() {
		tarball, err := archive.Create(t.Path)
		if err != nil {
			errch <- err
			return
		}

		// Construct and deploy ConfigMap resource
		configmap, err := t.createConfigMap(rc, tarball, name, v1alpha1.RunDefaultConfigMapKey)
		if err != nil {
			errch <- err
			return
		} else {
			resources = append(resources, configmap)
			uploaded <- struct{}{}
		}
	}()

	// Construct and deploy command resource
	run, err := t.createRun(rc, name, name)
	if err != nil {
		return err
	}
	resources = append(resources, run)

	// Block until archive successsfully uploaded
	select {
	case <-uploaded:
		// proceed
	case err := <-errch:
		return err
	}

	// Instantiate k8s cache mgr
	mgr, err := t.Factory.NewManager(config, t.Namespace)
	if err != nil {
		return err
	}

	// Run workspace reporter, which reports on the run's queue position
	mgr.AddReporter(&reporters.WorkspaceReporter{
		Client: rc,
		Id:     t.Workspace,
		CmdId:  name,
	})

	// Run run reporter, which waits until the cmd status reports the pod is ready
	mgr.AddReporter(&reporters.RunReporter{
		Client:         rc,
		Id:             name,
		EnqueueTimeout: t.TimeoutEnqueue,
		QueueTimeout:   t.TimeoutQueue,
	})

	// Run cache mgr, blocking until the run reporter returns successfully, indicating that we can
	// proceed to connecting to the pod.
	if err := mgr.Start(ctx); err != nil {
		return err
	}

	// Get pod
	pod := &corev1.Pod{}
	if err := rc.Get(ctx, types.NamespacedName{Name: name, Namespace: t.Namespace}, pod); err != nil {
		return err
	}

	podlog := log.WithField("pod", k8s.GetNamespacedName(pod))

	// Fetch pod's log stream
	podlog.Debug("retrieve log stream")
	logstream, err := rc.GetLogs(t.Namespace, name, &corev1.PodLogOptions{Follow: true})
	if err != nil {
		return err
	}
	defer logstream.Close()

	// Attach to pod tty, falling back to log stream upon error
	errors := make(chan error)
	go func() {
		podlog.Debug("attaching")
		errors <- k8s.AttachFallbackToLogs(rc, pod, logstream)
	}()

	// Let operator know we're now at least streaming logs (so if there is an error message then at least
	// it'll be fully streamed to the client)
	if err := k8s.ReleaseHold(ctx, rc, run); err != nil {
		return err
	}

	// Wait until attach returns
	return <-errors
}
