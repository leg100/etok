package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1/command"
	"github.com/leg100/stok/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type runnerCmd struct {
	Name           string
	Namespace      string
	Path           string
	KubeConfigPath string
	TimeoutClient  time.Duration
	Tarball        string
	Kind           string
	Semaphore      bool

	args    []string
	current string
	scheme  *runtime.Scheme
	cmd     *cobra.Command
}

func newRunnerCmd() *cobra.Command {
	runner := &runnerCmd{}

	cmd = &cobra.Command{
		// TODO: what is the syntax for stating at least one command must be provided?
		Use:           "runner [command...]",
		Short:         "Run the stok runner",
		Long:          "The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("command needs to be provided")
			}
			runner.args = args

			if runner.Tarball != "" {
				files, err := extractTarball(runner.Tarball, runner.Path)
				if err != nil {
					return err
				}
				log.WithFields(log.Fields{"files": files, "path": runner.Path}).Debug("extracted tarball")
			}

			if runner.Semaphore {
				if err := runner.handleSemaphore(); err != nil {
					return err
				}
			}

			if err := run(args, runner.Path, os.Stdout, os.Stderr); err != nil {
				return err
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&runner.Path, "path", ".", "Workspace config path")
	cmd.Flags().StringVar(&runner.Tarball, "tarball", "", "Extract specified tarball file to workspace path")

	// TODO: The runner currently expects to wait for the command resource's ClientReady condition
	// to be true. In order to do this, it needs the args below.
	// However, in future, we want to make this optional, i.e. so that the stok CLI is not
	// mandatory, and that one can just create custom resources directly. In this case, this runner
	// would not wait for the resource, and would not need these parameters. Indeed, it would not
	// need to talk to the k8s api at all.
	cmd.Flags().BoolVar(&runner.Wait, "wait", false, "Require runner to wait for the annotation to be set before proceeding")
	cmd.Flags().StringVar(&runner.WaitName, "wait-name", "", "Name of command resource with annotation")
	cmd.Flags().StringVar(&runner.WaitNamespace, "wait-namespace", "default", "Namespace of command resource with annotation")
	cmd.Flags().StringVar(&runner.WaitKind, "wait-kind", "", "Kind of command resource with annotation.")
	cmd.Flags().DurationVar(&runner.WaitTimeout, "wait-timeout", 10*time.Second, "timeout for client to signal readiness")

	// TODO: Document that this arg only comes into effect if the runner detects that it is *not*
	// running on a cluster.
	cmd.Flags().StringVar(&runner.KubeConfigPath, "kubeconfig", "", "absolute path to kubeconfig file (default is $HOME/.kube/config)")

	runner.cmd = cmd
	return runner.cmd
}

//func (r *runnerCmd) buildArgs(tarball, k

// Extract Tarball with path 'src' to path 'dst'
func extractTarball(src, dst string) (int, error) {
	tarBytesBuffer := new(bytes.Buffer)
	tarBytes, err := ioutil.ReadFile(src)
	if err != nil {
		return 0, err
	}

	_, err = tarBytesBuffer.Write(tarBytes)
	if err != nil {
		return 0, err
	}

	return util.Extract(tarBytesBuffer, dst)
}

func handleSemaphore(kind, name, namespace string, timeout time.Duration) error {
	// Get built-in scheme
	s := scheme.Scheme
	// And add our CRDs
	apis.AddToScheme(s)

	config, err := rest.InClusterConfig()
	if err == rest.ErrNotInCluster {
		log.Warn("Not running on k8s cluster")
		config, err = configFromPath(t.KubeConfigPath)
		if err != nil {
			return err
		}
	}

	// Get REST Client for listwatch for watching command resource
	gvk := v1alpha1.SchemeGroupVersion.WithKind(kind)
	rc, err := apiutil.RESTClientForGVK(gvk, config, serializer.NewCodecFactory(s))
	if err != nil {
		return err
	}

	obj, err := s.New(gvk)
	if err != nil {
		return err
	}
	c := obj.(command.Interface)

	plural := v1alpha1.CmdCRD(kind).Plural()
	_, err = waitUntil(rc, c, name, namespace, plural, commandCheckointCleared, timeout)
	if err != nil {
		return err
	}

	return nil
}

// A watchtools.ConditionFunc that returns true when a Command resource's checkpoint annotation is
// cleared, false otherwise
func commandCheckointCleared(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, fmt.Errorf("command not found")
	}
	switch t := event.Object.(type) {
	case command.Interface:
		_, ok := t.GetAnnotations()[v1alpha1.CommandWaitAnnotationKey]
		if !ok {
			// No checkpoint annotation set, we're clear to go
			return true, nil
		}
	}
	return false, nil
}

// Run args, taking first arg as executable, and remainder as args to executable. Path sets the
// working directory of the executable; out and errout set stdout and stderr of executable.
func run(args []string, path string, out, errout io.Writer) error {
	exe := exec.Command(args[0], args[1:]...)
	exe.Dir = path
	exe.Stdout = out
	exe.Stderr = errout

	return exe.Run()
}
