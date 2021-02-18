package github

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestWebhookServer(t *testing.T) {
	opts := webhookServerOptions{
		cloneDir: testutil.NewTempDir(t).Root(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errch := make(chan error)
	go func() {
		errch <- opts.run(ctx)
	}()

	repo, headSHA := initializeRepo(&testutil.T{T: t})

	require.NoError(t, <-errch)
}

func initializeRepo(t *testutil.T) (string, string) {
	repo := t.NewTempDir().Root()
	runCmdInRepo(t, repo, "git", "init")
	runCmdInRepo(t, repo, "touch", ".gitkeep")
	runCmdInRepo(t, repo, "git", "add", ".gitkeep")

	runCmdInRepo(t, repo, "git", "config", "--local", "user.email", "etok@etok.dev")
	runCmdInRepo(t, repo, "git", "config", "--local", "user.name", "etok")
	runCmdInRepo(t, repo, "git", "commit", "-m", "initial commit")
	runCmdInRepo(t, repo, "git", "checkout", "-b", "branch")
	runCmdInRepo(t, repo, "git", "add", ".")
	runCmdInRepo(t, repo, "git", "commit", "-am", "branch commit")
	headSHA := runCmdInRepo(t, repo, "git", "rev-parse", "HEAD")
	headSHA = strings.Trim(headSHA, "\n")

	return repo, headSHA
}

func runCmdInRepo(t *testutil.T, dir string, name string, args ...string) string {
	cpCmd := exec.Command(name, args...)
	cpCmd.Dir = dir
	cpOut, err := cpCmd.CombinedOutput()
	require.NoError(t, err)
	return string(cpOut)
}
