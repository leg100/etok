package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/executor"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/launcher"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultNamespace = "default"
)

type RunnerOptions struct {
	*cmdutil.Factory

	*client.Client

	path        string
	tarball     string
	dest        string
	command     string
	namespace   string
	kubeContext string

	runName string

	exec executor.Executor

	handshake        bool
	handshakeTimeout time.Duration

	args []string
}

func RunnerCmd(opts *cmdutil.Factory) (*cobra.Command, *RunnerOptions) {
	o := &RunnerOptions{
		Factory: opts,
		exec:    &executor.Exec{IOStreams: opts.IOStreams},
	}

	cmd := &cobra.Command{
		Use:    "runner [args]",
		Short:  "Run the etok runner",
		Long:   "Runner runs the requested command on a Run's pod. Prior to running the command, it can optionally be requested to untar a tarball into a destination directory, and it can optionally be requested to await a 'handshake' on stdin - a string a client can send to inform the runner it has successfully attached to the pod's TTY, ensuring it doesn't miss any output from the command that the runner then runs.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			if err := o.validate(); err != nil {
				return prefixError(err)
			}

			o.args = args

			o.Client, err = opts.Create(o.kubeContext)
			if err != nil {
				return err
			}

			return prefixError(o.Run(cmd.Context()))
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	cmd.Flags().StringVar(&o.dest, "dest", "/workspace", "Destination path for tarball extraction")
	cmd.Flags().StringVar(&o.tarball, "tarball", o.tarball, "Tarball filename")
	cmd.Flags().BoolVar(&o.handshake, "handshake", false, "Await handshake string on stdin")
	cmd.Flags().DurationVar(&o.handshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "Timeout waiting for handshake")
	cmd.Flags().StringVar(&o.runName, "run-name", "", "Name of run resource")
	cmd.Flags().StringVar(&o.command, "command", "", "Etok command to run")

	return cmd, o
}

func prefixError(err error) error {
	if err != nil {
		return fmt.Errorf("[runner] %w", err)
	}
	return nil
}

func (o *RunnerOptions) validate() error {
	if o.namespace == "" {
		return errors.New("--namespace cannot be empty")
	}

	if o.command == "" {
		return errors.New("--command cannot be empty")
	}

	if launcher.UpdatesLockFile(o.command) {
		if o.runName == "" {
			return fmt.Errorf("%s updates lock file; --run-name cannot be empty", o.command)
		}
	}

	return nil
}

func (o *RunnerOptions) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	// Concurrently extract tarball
	if o.tarball != "" {
		g.Go(func() error {
			f, err := os.Open(o.tarball)
			if err != nil {
				return fmt.Errorf("failed to open tarball: %w", err)
			}
			defer f.Close()

			if err := archive.Unpack(f, o.dest); err != nil {
				return fmt.Errorf("failed to extract tarball: %w", err)
			}

			return nil
		})
	}

	// Concurrently wait for client to handshake
	if o.handshake {
		g.Go(func() error {
			return o.withRawMode(gctx, o.receiveHandshake)
		})
	}

	// Wait for concurrent tasks to complete
	if err := g.Wait(); err != nil {
		return err
	}

	// Execute requested command
	if err := o.exec.Execute(ctx, prepareArgs(o.command, o.args...)); err != nil {
		return err
	}

	if launcher.UpdatesLockFile(o.command) {
		// This is a command that updates the lock file (such as terraform init)
		// so persist it to a configmap
		if err := o.persistLockFile(ctx); err != nil {
			return fmt.Errorf("failed to persist lock file to config map: %w", err)
		}
	}

	return nil
}

// persistLockFile persists the lock file .terraform.lock.hcl to a config map.
// If the lock file does not exist then it exits early without error.
func (o *RunnerOptions) persistLockFile(ctx context.Context) error {
	// Check if file exists
	lockFileContents, err := os.ReadFile(globals.LockFile)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(1).Infof("%s not found", globals.LockFile)
			return nil
		}
	}

	// File exists so continue...

	// Get run resource so that it can be set as owner of config map
	run, err := o.RunsClient(o.namespace).Get(ctx, o.runName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to retrieve run: %w", err)
	}

	// create configmap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.namespace,
			Name:      v1alpha1.RunLockFileConfigMapName(o.runName),
		},
		BinaryData: map[string][]byte{
			globals.LockFile: lockFileContents,
		},
	}
	// Set etok's common labels
	labels.SetCommonLabels(configMap)
	// Permit filtering archives by command
	labels.SetLabel(configMap, labels.Command(o.command))
	// Permit filtering etok resources by component
	labels.SetLabel(configMap, labels.RunComponent)

	// Make run owner of configmap, so if run is deleted so is its configmap
	if err := controllerutil.SetOwnerReference(run, configMap, scheme.Scheme); err != nil {
		return err
	}

	_, err = o.ConfigMapsClient(o.namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	klog.V(1).Infof("created config map: %s", klog.KObj(configMap))
	return nil
}

// Set TTY in raw mode for the duration of running f.
func (o *RunnerOptions) withRawMode(ctx context.Context, f func(context.Context) error) error {
	// Set stdin in raw mode.
	oldState, err := terminal.MakeRaw(int(o.In.(*os.File).Fd()))
	if err != nil {
		return fmt.Errorf("failed to put TTY into raw mode: %w", err)
	}
	defer func() {
		if err = terminal.Restore(int(o.In.(*os.File).Fd()), oldState); err != nil {
			klog.V(1).Infof("[runner] failed to restore TTY\r\n")
		} else {
			klog.V(1).Infof("[runner] restored TTY\r\n")
		}
	}()

	return f(ctx)
}

// Receive handshake string from client. If handshake string is not received
// within timeout, or anything other than handshake string is received, then an
// error is returned.
func (o *RunnerOptions) receiveHandshake(ctx context.Context) error {
	buf := make([]byte, len(cmdutil.HandshakeString))
	errch := make(chan error)
	// In raw mode both carriage return and newline characters are necessary
	klog.V(1).Infof("[runner] awaiting handshake\r\n")

	// FIXME: Occasionally read() blocks awaiting a newline, despite stdin being in raw mode. I
	// suspect terminal.MakeRaw is only asynchronously taking affect, and the stdin is
	// sometimes still in cooked mode.
	go func() {
		var read int // tally of bytes read so far
		for {
			n, err := o.In.Read(buf[read:])
			read += n
			if read == len(buf) {
				errch <- nil
			} else if err == io.EOF {
				errch <- fmt.Errorf("handshake: reached EOF")
			} else if err != nil {
				errch <- fmt.Errorf("handshake: %w", err)
			} else {
				continue
			}
			return
		}
	}()

	select {
	case <-time.After(o.handshakeTimeout):
		return errHandshakeTimeout
	case err := <-errch:
		if err != nil {
			return err
		}
		if string(buf) != cmdutil.HandshakeString {
			return errIncorrectHandshake
		}
	}
	klog.V(1).Infof("[runner] handshake completed\r\n")
	return nil
}

var (
	errIncorrectHandshake = errors.New("incorrect handshake received")
	errHandshakeTimeout   = errors.New("timed out awaiting handshake")
)
