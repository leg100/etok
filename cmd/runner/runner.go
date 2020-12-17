package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/launcher"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/util/slice"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/errgroup"
	"k8s.io/klog/v2"
)

type RunnerOptions struct {
	*cmdutil.Options

	Path      string
	Tarball   string
	Dest      string
	Command   string
	Workspace string

	exec Executor

	Handshake        bool
	HandshakeTimeout time.Duration

	args []string
}

func RunnerCmd(opts *cmdutil.Options) (*cobra.Command, *RunnerOptions) {
	o := &RunnerOptions{Options: opts, exec: &executor{IOStreams: opts.IOStreams}}
	cmd := &cobra.Command{
		Use:    "runner [args]",
		Short:  "Run the etok runner",
		Long:   "Runner runs the requested command on a Run's pod. Prior to running the command, it can optionally be requested to untar a tarball into a destination directory, and it can optionally be requested to await a 'handshake' on stdin - a string a client can send to inform the runner it has successfully attached to the pod's TTY, ensuring it doesn't miss any output from the command that the runner then runs.",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.args = args

			return prefixError(o.Run(cmd.Context()))
		},
	}

	cmd.Flags().StringVar(&o.Dest, "dest", "/workspace", "Destination path for tarball extraction")
	cmd.Flags().StringVar(&o.Tarball, "tarball", o.Tarball, "Tarball filename")
	cmd.Flags().StringVar(&o.Command, "command", "", "Etok command to run")
	cmd.Flags().StringVar(&o.Workspace, "workspace", "", "Terraform workspace")
	cmd.Flags().BoolVar(&o.Handshake, "handshake", false, "Await handshake string on stdin")
	cmd.Flags().DurationVar(&o.HandshakeTimeout, "handshake-timeout", v1alpha1.DefaultHandshakeTimeout, "Timeout waiting for handshake")

	return cmd, o
}

func prefixError(err error) error {
	if err != nil {
		return fmt.Errorf("[runner] %w", err)
	}
	return nil
}

func (o *RunnerOptions) Run(ctx context.Context) error {
	// Validate command
	if !slice.ContainsString(launcher.Cmds.GetNames(), o.Command) {
		return fmt.Errorf("%s: %w", o.Command, errUnknownCommand)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Concurrently extract tarball
	if o.Tarball != "" {
		g.Go(func() error {
			f, err := os.Open(o.Tarball)
			if err != nil {
				return fmt.Errorf("failed to open tarball: %w", err)
			}
			defer f.Close()

			if err := archive.Unpack(f, o.Dest); err != nil {
				return fmt.Errorf("failed to extract tarball: %w", err)
			}

			return nil
		})
	}

	// Concurrently wait for client to handshake
	if o.Handshake {
		g.Go(func() error {
			return o.withRawMode(gctx, o.handshake)
		})
	}

	// Wait for concurrent tasks to complete
	if err := g.Wait(); err != nil {
		return err
	}

	// Execute requested command
	switch o.Command {
	case "sh":
		return o.exec.run(ctx, append([]string{"sh"}, o.args...))
	default:
		return execTerraformRun(ctx, o.exec, o.Command, o.Workspace, o.args)
	}
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

// Receive handshake string from client. If handshake string is not received within
// timeout, or anything other than handshake string is received, then an error is
// returned.
func (o *RunnerOptions) handshake(ctx context.Context) error {
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
	case <-time.After(o.HandshakeTimeout):
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
	errUnknownCommand     = errors.New("unknown command")
)
