package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/leg100/stok/cmd/flags"
	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/pkg/archive"
	"github.com/leg100/stok/pkg/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/sync/errgroup"
)

const (
	defaultTimeoutClient = 10 * time.Second
)

type RunnerOptions struct {
	*cmdutil.Options

	Path               string
	Tarball            string
	RequireMagicString bool

	TimeoutClient time.Duration

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

	cmd.Flags().StringVar(&o.Tarball, "tarball", o.Tarball, "Tarball filename")
	cmd.Flags().BoolVar(&o.RequireMagicString, "require-magic-string", false, "Await magic string on stdin")
	cmd.Flags().DurationVar(&o.TimeoutClient, "timeout", defaultTimeoutClient, "Timeout for client to signal readiness")

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
			_, err := archive.Extract(o.Tarball, o.Path)
			return err
		})
	}

	// Concurrently wait for client to send magic string
	if o.RequireMagicString {
		g.Go(func() error {
			return o.withRawMode(gctx, o.receiveMagicString)
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

// Receive magic string from client. If magic string is not received within
// timeout, or anything other than magic string is received, then an error is
// returned.
func (o *RunnerOptions) receiveMagicString(ctx context.Context) error {
	buf := make([]byte, len(cmdutil.MagicString))
	errch := make(chan error)

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
				errch <- fmt.Errorf("reached EOF while reading magic string")
			} else if err != nil {
				errch <- fmt.Errorf("encountered error reading magic string: %w", err)
			} else {
				continue
			}
			return
		}
	}()

	select {
	case <-time.After(o.TimeoutClient):
		return fmt.Errorf("timed out waiting for magic string")
	case err := <-errch:
		if err != nil {
			return err
		}
		if string(buf) != cmdutil.MagicString {
			return fmt.Errorf("expected magic string '%s' but received: %s", cmdutil.MagicString, string(buf))
		}
	}
	log.Debugf("[runner] magic string received\r\n")
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
