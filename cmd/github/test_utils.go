package github

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func initializeRepo(t *testutil.T, seed string) (string, string) {
	// Create a temp dir for the repo. Workspaces in the test use the repository
	// identifier "bob/myrepo", so we need to ensure the repo url matches this,
	// i.e. file://tmp/.../bob/myrepo.git
	tmpdir := t.NewTempDir().Mkdir("bob/myrepo.git")
	repo := filepath.Join(tmpdir.Root(), "bob", "myrepo.git")

	seedAbsPath, err := filepath.Abs(seed)
	require.NoError(t, err)

	runCmdInRepo(t, "", "cp", "-a", seedAbsPath+"/.", repo)

	runCmdInRepo(t, repo, "git", "init")
	runCmdInRepo(t, repo, "touch", ".gitkeep")
	runCmdInRepo(t, repo, "git", "add", ".gitkeep")

	runCmdInRepo(t, repo, "git", "config", "--local", "user.email", "etok@etok.dev")
	runCmdInRepo(t, repo, "git", "config", "--local", "user.name", "etok")
	runCmdInRepo(t, repo, "git", "commit", "-m", "initial commit")
	runCmdInRepo(t, repo, "git", "checkout", "-b", "changes")
	runCmdInRepo(t, repo, "git", "add", ".")
	runCmdInRepo(t, repo, "git", "commit", "-am", "changes commit")
	headSHA := runCmdInRepo(t, repo, "git", "rev-parse", "HEAD")
	headSHA = strings.Trim(headSHA, "\n")

	return repo, headSHA
}

func runCmdInRepo(t *testutil.T, dir string, name string, args ...string) string {
	cpCmd := exec.Command(name, args...)
	cpCmd.Dir = dir
	cpOut, err := cpCmd.CombinedOutput()
	if err != nil {
		t.Errorf("%s %s failed: %s", name, args, cpOut)
	}
	return string(cpOut)
}
