package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type FakeExecutor struct{}

func (fe *FakeExecutor) Execute(ctx context.Context, args []string, opts ...ExecOption) error {
	return nil
}

// Fake that prints any args to stdout
type FakeExecutorEchoArgs struct {
	Out io.Writer
}

func (fe *FakeExecutorEchoArgs) Execute(ctx context.Context, args []string, opts ...ExecOption) error {
	fmt.Fprintf(fe.Out, "%v", args)
	return nil
}

type FakeExecutorMissingWorkspace struct {
	Out io.Writer
}

func (fe *FakeExecutorMissingWorkspace) Execute(ctx context.Context, args []string, opts ...ExecOption) error {
	fmt.Fprintf(fe.Out, "%v", args)

	if len(args) > 2 && args[0] == "terraform" && args[1] == "workspace" && args[2] == "select" {
		return errors.New("workspace does not exist")
	}

	return nil
}
