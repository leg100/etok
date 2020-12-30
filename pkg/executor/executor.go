package executor

import (
	"context"
	"fmt"
	"os/exec"

	cmdutil "github.com/leg100/etok/cmd/util"
	"k8s.io/klog/v2"
)

type Executor interface {
	Execute(context.Context, []string, ...ExecOption) error
}

type ExecOption func(*exec.Cmd)

type Exec struct {
	cmdutil.IOStreams
}

func (tc *Exec) Execute(ctx context.Context, args []string, opts ...ExecOption) error {
	klog.V(1).Infof("running command %v\n", args)

	exe := exec.CommandContext(ctx, args[0], args[1:]...)
	exe.Stdin = tc.In
	exe.Stdout = tc.Out
	exe.Stderr = tc.ErrOut

	for _, o := range opts {
		o(exe)
	}

	if err := exe.Run(); err != nil {
		return fmt.Errorf("unable to run command %v: %w", args, err)
	}
	return nil
}

func withPath(path string) ExecOption {
	return func(cmd *exec.Cmd) {
		cmd.Dir = path
	}
}
