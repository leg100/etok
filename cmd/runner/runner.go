package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/errgroup"
)

type RunnerOptions struct {
	*cmdutil.Options

	Path    string
	Tarball string
	Dest    string

	Handshake        bool
	HandshakeTimeout time.Duration

	args []string
}

func RunnerCmd(opts *cmdutil.Options) (*cobra.Command, *RunnerOptions) {
	o := &RunnerOptions{Options: opts}
	cmd := &cobra.Command{
		Use:    "runner [command (args)]",
		Short:  "Run the stok runner",
		Long:   "The stok runner is intended to be run in on pod, started by the relevant stok command controller. When invoked, it extracts a tarball containing terraform configuration files. It then waits for the command's ClientReady condition to be true. And then it invokes the relevant command, typically a terraform command.",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.args = args

			return prefixError(o.Run(cmd.Context()))
		},
	}

	flags.AddPathFlag(cmd, &o.Path)

	cmd.Flags().StringVar(&o.Dest, "dest", "/workspace", "Destination path for tarball extraction")
	cmd.Flags().StringVar(&o.Tarball, "tarball", o.Tarball, "Tarball filename")
	cmd.Flags().BoolVar(&o.Handshake, "handshake", false, "Await handshake string on stdin")
	cmd.Flags().DurationVar(&o.HandshakeTimeout, "timeout", v1alpha1.DefaultHandshakeTimeout, "Timeout waiting for handshake")

	return cmd, o
}

func prefixError(err error) error {
	if err != nil {
		return fmt.Errorf("[runner] %w", err)
	}
	return nil
}

func (o *RunnerOptions) Run(ctx context.Context) error {
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

	return o.run(ctx, o.Out, os.Stderr)
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
			log.Debugf("[runner] failed to restore TTY\r\n")
		} else {
			log.Debugf("[runner] restored TTY\r\n")
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
	log.Debugf("[runner] awaiting handshake\r\n")

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
		return fmt.Errorf("timed out waiting for handshake")
	case err := <-errch:
		if err != nil {
			return err
		}
		if string(buf) != cmdutil.HandshakeString {
			return fmt.Errorf("handshake: expected '%s' but received: %s", cmdutil.HandshakeString, string(buf))
		}
	}
	log.Debugf("[runner] handshake completed\r\n")
	return nil
}

// Run args, taking first arg as executable, and remainder as args to executable. Path sets the
// working directory of the executable; out and errout set stdout and stderr of executable.
func (o *RunnerOptions) run(ctx context.Context, out, errout io.Writer) error {
	return Run(ctx, o.args, o.Path, out, errout)
}

// Synchronously run command, taking first arg of args as executable, and remainder as arguments.
func Run(ctx context.Context, args []string, path string, out, errout io.Writer) error {
	log.Debugf("[runner] running command %v\n", args)

	exe := exec.CommandContext(ctx, args[0], args[1:]...)
	exe.Dir = path
	exe.Stdin = os.Stdin
	exe.Stdout = out
	exe.Stderr = errout

	return exe.Run()
}
