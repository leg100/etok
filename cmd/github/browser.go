package github

import (
	"context"
	"os/exec"
	"runtime"
)

func getOpener() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"cmd", "/c", "start"}
	case "darwin":
		return []string{"open"}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		return []string{"xdg-open"}
	}
}

type runnerInterface interface {
	run(context.Context, ...string) error
}

type runner struct{}

func (runner) run(ctx context.Context, args ...string) error {
	return exec.CommandContext(ctx, args[0], args[1:]...).Start()
}

type fakeRunner struct{}

func (fakeRunner) run(ctx context.Context, args ...string) error {
	return nil
}
