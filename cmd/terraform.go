package cmd

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/util/slice"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type terraformCmd struct {
	Workspace      string
	Namespace      string
	Context        string
	Path           string
	Args           []string
	Kind           string
	TimeoutClient  time.Duration
	TimeoutPod     time.Duration
	TimeoutQueue   time.Duration
	TimeoutEnqueue time.Duration

	cmd *cobra.Command

	debug   bool
	factory k8s.FactoryInterface
}

func newTerraformCmds(f k8s.FactoryInterface) []*cobra.Command {
	var cmds []*cobra.Command

	for _, kind := range command.CommandKinds {
		cc := &terraformCmd{}
		cc.Kind = kind
		cc.cmd = &cobra.Command{
			Use:   command.CommandKindToCLI(kind),
			Short: fmt.Sprintf("Run %s", command.CommandKindToCLI(kind)),
			RunE:  cc.doTerraformCmd,
		}
		cc.cmd.Flags().StringVar(&cc.Path, "path", ".", "terraform config path")
		cc.cmd.Flags().DurationVar(&cc.TimeoutPod, "timeout-pod", time.Minute, "timeout for pod to be ready and running")
		cc.cmd.Flags().DurationVar(&cc.TimeoutClient, "timeout-client", 10*time.Second, "timeout for client to signal readiness")
		cc.cmd.Flags().DurationVar(&cc.TimeoutQueue, "timeout-queue", time.Hour, "timeout waiting in workspace queue")
		cc.cmd.Flags().StringVar(&cc.Namespace, "namespace", "", "Kubernetes namespace of workspace (defaults to namespace set in .terraform/environment, or \"default\")")

		cc.cmd.Flags().StringVar(&cc.Workspace, "workspace", "", "Workspace name (defaults to workspace set in .terraform/environment or, \"default\")")
		cc.cmd.Flags().StringVar(&cc.Context, "context", "", "Set kube context (defaults to kubeconfig current context)")

		// Add flags registered by imported packages (controller-runtime)
		cc.cmd.Flags().AddGoFlagSet(flag.CommandLine)

		cc.factory = f

		cmds = append(cmds, cc.cmd)
	}

	return cmds
}

func (t *terraformCmd) doTerraformCmd(cmd *cobra.Command, args []string) error {
	debug, err := cmd.InheritedFlags().GetBool("debug")
	if err != nil {
		return err
	}
	t.debug = debug

	// TODO: remove
	if err := unmarshalV(t); err != nil {
		return err
	}

	t.Args = args

	// Workspace config precedence:
	// 1. Flag
	// 2. Environment File
	// 3. "default"
	if t.Workspace == "" {
		_, workspace, err := readEnvironmentFile(t.Path)
		if errors.IsNotFound(err) {
			t.Workspace = "default"
		} else if err != nil {
			return err
		}
		t.Workspace = workspace
	}

	// Namespace config precedence:
	// 1. Flag
	// 2. Environment File
	// 3. "default"
	if t.Namespace == "" {
		namespace, _, err := readEnvironmentFile(t.Path)
		if errors.IsNotFound(err) {
			t.Namespace = "default"
		} else if err != nil {
			return err
		}
		t.Namespace = namespace
	}

	return t.run()
}

// create dynamic client
// exit if workspace is unhealthy
// construct command resource
// create tarball
// embed tarball in configmap
// deploy resources
// watch queue until we are front of queue
// get logs
// get pod logs stream
// attach to pod (falling back to logs on error)
func (t *terraformCmd) run() error {
	// Get client from factory. Embeds controller-runtime client
	rc, err := t.factory.NewClient(t.Context)
	if err != nil {
		return err
	}

	// Get client from factory. Embeds controller-runtime client
	cache, err := rc.NewCache(t.Namespace)
	if err != nil {
		return err
	}

	// Generate unique name shared by command and configmap resources (and command ctrl will spawn a
	// pod with this name, too)
	name := command.GenerateName(t.Kind)

	var quit = make(chan struct{})
	var done = make(chan error)
	var resources []runtime.Object
	var cmd command.Interface
	var configmap *corev1.ConfigMap

	// Delete resources upon program termination.
	defer func() {
		// Deleting cmd will delete any pod it creates too
		rc.Delete(context.TODO(), cmd)
		rc.Delete(context.TODO(), configmap)
	}()

	// Compile tarball of terraform module, embed in configmap and deploy
	go func() {
		tarball, err := t.createTar()
		if err != nil {
			done <- err
			return
		}

		// TODO: just let the create configmap step throw an error rather than pre-empting it.
		if len(tarball) > v1alpha1.MaxConfigSize {
			done <- fmt.Errorf("max config size exceeded; current=%d; max=%d", len(tarball), v1alpha1.MaxConfigSize)
			return
		}

		// Construct and deploy ConfigMap resource
		configmap, err := t.createConfigMap(rc, tarball, name, v1alpha1.CommandDefaultConfigMapKey)
		if err != nil {
			done <- err
			return
		} else {
			resources = append(resources, configmap)
		}
		done <- nil
	}()

	// Construct and deploy command resource
	cmd, err = t.createCommand(rc, name, name)
	if err != nil {
		return err
	}

	events := t.factory.GetEventsChannel()

	cmdInformer, err := cache.GetInformerForKind(context.TODO(), v1alpha1.SchemeGroupVersion.WithKind(t.Kind))
	if err != nil {
		return err
	}
	cmdInformer.AddEventHandler(k8s.EventHandlers(events))

	workspaceInformer, err := cache.GetInformer(context.TODO(), &v1alpha1.Workspace{})
	if err != nil {
		return err
	}
	workspaceInformer.AddEventHandler(k8s.EventHandlers(events))

	podInformer, err := cache.GetInformer(context.TODO(), &corev1.Pod{})
	if err != nil {
		return err
	}
	podInformer.AddEventHandler(k8s.EventHandlers(events))

	go func() {
		if err := cache.Start(quit); err != nil {
			done <- err
		}
	}()

	enqueueTimer := time.NewTimer(t.TimeoutEnqueue)
	queueTimer := time.NewTimer(t.TimeoutQueue)
	var phase v1alpha1.CommandPhase

	var pod *corev1.Pod
	var podRunningAndReady bool
	for {
		var err = error(nil)

		select {
		case <-enqueueTimer.C:
			err = fmt.Errorf("timeout reached for enqueuing command")
		case <-queueTimer.C:
			err = fmt.Errorf("timeout reached for queued command")
		case e := <-events:
			switch obj := e.(type) {
			case command.Interface:
				phase, err = t.CommandReporter(rc, obj, name)
			case *v1alpha1.Workspace:
				err = t.WorkspaceReporter(rc, obj, name)
			case *corev1.Pod:
				podRunningAndReady, err = t.PodReporter(rc, obj, name)
				if podRunningAndReady {
					pod = obj
				}
			}
		case err = <-done:
			return err
		}

		if err != nil {
			close(quit)
			return err
		}

		if podRunningAndReady {
			// Proceed to stream logs / attach to tty
			break
		}

		switch phase {
		case v1alpha1.CommanndPhaseQueued:
			if !enqueueTimer.Stop() {
				<-enqueueTimer.C
			}
		case v1alpha1.CommanndPhaseActive:
			if !queueTimer.Stop() {
				<-queueTimer.C
			}
		}
	}

	log.WithFields(log.Fields{"pod": name, "namespace": t.Namespace}).Debug("retrieve log stream")
	logs, err := rc.GetLogs(t.Namespace, name, &corev1.PodLogOptions{Follow: true})
	if err != nil {
		return err
	}
	defer logs.Close()

	// Attach to pod tty
	errors := make(chan error)
	go func() {
		log.WithFields(log.Fields{"pod": name}).Debug("attaching")
		errors <- k8s.AttachFallbackToLogs(rc, pod, logs)
	}()

	// Let operator know we're now streaming logs
	if err := k8s.ReleaseHold(rc, cmd); err != nil {
		return err
	}

	return <-errors
}

func (t *terraformCmd) WorkspaceReporter(rc k8s.Client, obj interface{}, cmdname string) error {
	req := k8s.GetNamespacedName(obj.(metav1.Object))

	if req.Name != t.Workspace {
		// This is for another workspace
		return nil
	}

	wslog := log.WithField("workspace", req)

	// Fetch the Workspace instance
	// Check workspace resource exists and is healthy
	ws := &v1alpha1.Workspace{}
	if err := rc.Get(context.TODO(), req, ws); err != nil {
		// Workspace not found or some other error. Either way, it's bad.
		// TODO: apart from transitory errors, which we could try to recover from.
		return err
	}
	wslog.Debug("existence confirmed")

	// Report on queue position
	if pos := slice.StringIndex(ws.Status.Queue, cmdname); pos >= 0 {
		// TODO: print cmdname in bold
		wslog.WithField("queue", ws.Status.Queue).Info("Queued")
	}

	return nil
}

func (t *terraformCmd) CommandReporter(rc k8s.Client, obj interface{}, name string) (v1alpha1.CommandPhase, error) {
	req := k8s.GetNamespacedName(obj.(metav1.Object))

	if req.Name != name {
		// This is for another command
		return "", nil
	}

	_ = log.WithField("command", req)

	cmd, err := command.NewCommandFromGVK(scheme.Scheme, v1alpha1.SchemeGroupVersion.WithKind(t.Kind))
	if err != nil {
		return "", err
	}

	// Fetch the Command instance
	if err := rc.Get(context.TODO(), req, cmd); err != nil {
		if errors.IsNotFound(err) {
			// Command yet to be created
			return "", nil
		}
		// Some error other than not found.
		// TODO: apart from transitory errors, which we could try to recover from.
		return "", err
	}

	// TODO: log status updates

	return cmd.GetPhase(), nil
}

// ErrPodCompleted is returned by PodRunning or PodContainerRunning to indicate that
// the pod has already reached completed state.
var ErrPodCompleted = fmt.Errorf("pod ran to completion")

func (t *terraformCmd) PodReporter(rc k8s.Client, obj interface{}, name string) (bool, error) {
	req := k8s.GetNamespacedName(obj.(metav1.Object))

	if req.Name != name {
		// This is for another pod
		return false, nil
	}

	podlog := log.WithField("pod", req)

	// Fetch the Pod instance
	pod := &corev1.Pod{}
	if err := rc.Get(context.TODO(), req, pod); err != nil {
		if errors.IsNotFound(err) {
			// Command yet to be created
			return false, nil
		}
		// Some error other than not found.
		// TODO: apart from transitory errors, which we could try to recover from.
		return false, err
	}

	podlog.WithFields(log.Fields{
		"namespace": t.Namespace,
	}).Debug("awaiting readiness")

	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return false, ErrPodCompleted
	case corev1.PodFailed:
		return false, fmt.Errorf(pod.Status.ContainerStatuses[0].State.Terminated.Message)
	case corev1.PodRunning:
		conditions := pod.Status.Conditions
		if conditions == nil {
			return false, nil
		}
		for i := range conditions {
			if conditions[i].Type == corev1.PodReady &&
				conditions[i].Status == corev1.ConditionTrue {
				return true, nil
			}
		}
	}
	return false, nil
}
