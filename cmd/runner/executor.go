package runner

import (
	"context"
	"fmt"
	"os/exec"

	cmdutil "github.com/leg100/etok/cmd/util"
	"k8s.io/klog/v2"
)

type Executor interface {
	run(context.Context, []string, ...execOption) error
}

type execOption func(*exec.Cmd)

type executor struct {
	cmdutil.IOStreams
}

func (tc *executor) run(ctx context.Context, args []string, opts ...execOption) error {
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

func withPath(path string) execOption {
	return func(cmd *exec.Cmd) {
		cmd.Dir = path
	}
}
