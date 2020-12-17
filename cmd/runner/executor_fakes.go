package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type fakeExecutor struct{}

func (fe *fakeExecutor) run(ctx context.Context, args []string, opts ...execOption) error {
	return nil
}

// Fake that prints any args to stdout
type fakeExecutorEchoArgs struct {
	out io.Writer
}

func (fe *fakeExecutorEchoArgs) run(ctx context.Context, args []string, opts ...execOption) error {
	fmt.Fprintf(fe.out, "%v", args)
	return nil
}

type fakeExecutorMissingWorkspace struct {
	out io.Writer
}

func (fe *fakeExecutorMissingWorkspace) run(ctx context.Context, args []string, opts ...execOption) error {
	fmt.Fprintf(fe.out, "%v", args)

	if len(args) > 2 && args[0] == "terraform" && args[1] == "workspace" && args[2] == "select" {
		return errors.New("workspace does not exist")
	}

	return nil
}
