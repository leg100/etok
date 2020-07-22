package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	"github.com/leg100/stok/util"
	"github.com/leg100/stok/util/slice"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type runnerCmd struct {
	Path    string
	Tarball string

	Name      string
	Namespace string
	Kind      string
	Timeout   time.Duration

	factory k8s.FactoryInterface
	args    []string
	cmd     *cobra.Command
}

func newRunnerCmd(f k8s.FactoryInterface) *cobra.Command {
	runner := &runnerCmd{}

	cmd := &cobra.Command{
		// TODO: what is the syntax for stating at least one command must be provided?
		Use:           "runner [command (args)]",
		Short:         "Run the stok runner",
		Long:          "The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runner.doRunnerCmd(args)
			if exiterr, ok := err.(*exec.ExitError); ok {
				return &componentError{
					err:       err,
					component: "runner",
					code:      exiterr.ExitCode(),
				}
			}
			if err != nil {
				return &componentError{
					err:       err,
					component: "runner",
					code:      1,
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&runner.Path, "path", ".", "Workspace config path")
	cmd.Flags().StringVar(&runner.Tarball, "tarball", "/tarball/tarball.tar.gz", "Extract specified tarball file to workspace path")

	cmd.Flags().StringVar(&runner.Name, "name", "", "Name of command resource")
	cmd.Flags().StringVar(&runner.Namespace, "namespace", "default", "Namespace of command resource")
	cmd.Flags().StringVar(&runner.Kind, "kind", "", "Kind of command resource")
	cmd.Flags().DurationVar(&runner.Timeout, "timeout", 10*time.Second, "Timeout on waiting for client to confirm readiness")

	runner.factory = f
	runner.cmd = cmd
	return runner.cmd
}

func (r *runnerCmd) doRunnerCmd(args []string) error {
	if err := r.validate(); err != nil {
		return err
	}

	r.args = args

	files, err := extractTarball(r.Tarball, r.Path)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{"files": files, "path": r.Path}).Debug("extracted tarball")

	// Get built-in scheme
	s := scheme.Scheme
	// And add our CRDs
	apis.AddToScheme(s)

	config, err := config.GetConfig()
	if err != nil {
		return err
	}

	rc, err := r.factory.NewClient(config, s)
	if err != nil {
		return err
	}

	if err := handleSemaphore(rc, s, r.Kind, r.Name, r.Namespace, r.Timeout); err != nil {
		return err
	}

	if err := r.run(os.Stdout, os.Stderr); err != nil {
		return err
	}

	return nil
}

func (r *runnerCmd) validate() error {
	if r.Kind == "" {
		return fmt.Errorf("missing flag: --kind <kind>")
	}

	if !slice.ContainsString(v1alpha1.CommandKinds, r.Kind) {
		return fmt.Errorf("invalid kind: %s", r.Kind)
	}

	return nil
}

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

func handleSemaphore(rc client.Client, s *runtime.Scheme, kind, name, namespace string, timeout time.Duration) error {
	// Get REST Client for listwatch for watching command resource
	gvk := v1alpha1.SchemeGroupVersion.WithKind(kind)

	// Wait until CommandWaitAnnotation is unset, or return error if timeout or other error occurs
	err := wait.Poll(500*time.Millisecond, timeout, func() (bool, error) {
		cmd, err := v1alpha1.NewCommandFromGVK(s, gvk)
		if err != nil {
			return false, err
		}
		if err := rc.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, cmd); err != nil {
			return false, err
		}
		if _, ok := cmd.GetAnnotations()[v1alpha1.CommandWaitAnnotationKey]; !ok {
			// No checkpoint annotation set, we're clear to go
			return true, nil
		}
		return false, nil
	})

	return err
}

// Run args, taking first arg as executable, and remainder as args to executable. Path sets the
// working directory of the executable; out and errout set stdout and stderr of executable.
func (r *runnerCmd) run(out, errout io.Writer) error {
	args := v1alpha1.RunnerArgsForKind(r.Kind, r.args)

	log.WithFields(log.Fields{"command": args[0], "args": fmt.Sprintf("%#v", args[1:])}).Debug("running command")

	exe := exec.Command(args[0], args[1:]...)
	exe.Dir = r.Path
	exe.Stdin = os.Stdin
	exe.Stdout = out
	exe.Stderr = errout

	return exe.Run()
}
