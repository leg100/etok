package cmd

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/api/command"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/pkg/k8s/reporters"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		// TODO: rename to timeout-pending (enqueue is too similar sounding to queue)
		cc.cmd.Flags().DurationVar(&cc.TimeoutEnqueue, "timeout-enqueue", 10*time.Second, "timeout waiting to be queued")
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
	// 2. Environment Variable
	// 3. Environment File
	// 4. "default"
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
	// 2. Environment Variable
	// 3. Environment File
	// 4. "default"
	if t.Namespace == "" {
		namespace, _, err := readEnvironmentFile(t.Path)
		if errors.IsNotFound(err) {
			t.Namespace = "default"
		} else if err != nil {
			return err
		}
		t.Namespace = namespace
	}

	return t.run(cmd.Context())
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
func (t *terraformCmd) run(ctx context.Context) error {
	config, err := t.factory.NewConfig(t.Context)
	if err != nil {
		return err
	}

	rc, err := t.factory.NewClient(config)
	if err != nil {
		return err
	}

	// Generate unique name shared by command and configmap resources (and command ctrl will spawn a
	// pod with this name, too)
	name := t.factory.GenerateName(t.Kind)

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
		configmap, err := t.createConfigMap(rc, tarball, name, v1alpha1.CommandDefaultConfigMapKey)
		if err != nil {
			errch <- err
			return
		} else {
			resources = append(resources, configmap)
			uploaded <- struct{}{}
		}
	}()

	// Construct and deploy command resource
	cmd, err := t.createCommand(rc, name, name)
	if err != nil {
		return err
	}
	resources = append(resources, cmd)

	// Block until archive successsfully uploaded
	select {
	case <-uploaded:
		// proceed
	case err := <-errch:
		return err
	}

	// Instantiate k8s cache mgr
	mgr, err := t.factory.NewManager(config, t.Namespace)
	if err != nil {
		return err
	}

	// Run workspace reporter, which reports on the cmd's queue position
	mgr.AddReporter(&reporters.WorkspaceReporter{
		Client: rc,
		Id:     t.Workspace,
		CmdId:  name,
	})

	// Run cmd reporter, which waits until the cmd status reports the pod is ready
	mgr.AddReporter(&reporters.CommandReporter{
		Client:         rc,
		Id:             name,
		Kind:           t.Kind,
		EnqueueTimeout: t.TimeoutEnqueue,
		QueueTimeout:   t.TimeoutQueue,
	})

	// Run cache mgr, blocking until the cmd reporter returns successfully, indicating that we can
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
	if err := k8s.ReleaseHold(ctx, rc, cmd); err != nil {
		return err
	}

	// Wait until attach returns
	return <-errors
}
