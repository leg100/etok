package e2e

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2E test of github webhook
func TestGithub(t *testing.T) {
	// Only run github tests on clusters exposed to internet, or when explicitly
	// asked to.
	if *kubectx == "kind-kind" && os.Getenv("GITHUB_E2E_TEST") != "true" {
		t.SkipNow()
	}

	// Path to cloned repo
	path := testutil.NewTempDir(t).Root()

	t.Parallel()
	t.Run("clone", func(t *testing.T) {
		require.NoError(t, exec.Command("git", "clone", os.Getenv("REPO_URL"), path).Run())
	})

	t.Run("create new branch", func(t *testing.T) {
		runWithPath(t, path, "git", "checkout", "-b", "e2e")
	})

	t.Run("write some terraform config", func(t *testing.T) {
		fpath := filepath.Join(path, "main.tf")
		require.NoError(t, ioutil.WriteFile(fpath, []byte("resource \"null_resource\" \"hello\" {}"), 0644))
	})

	t.Run("add terraform config file", func(t *testing.T) {
		runWithPath(t, path, "git", "add", "main.tf")
	})

	t.Run("commit terraform config file", func(t *testing.T) {
		runWithPath(t, path, "git", "commit", "-am", "e2e")
	})

	t.Run("push branch", func(t *testing.T) {
		runWithPath(t, path, "git", "push", "-f", "origin", "e2e")
	})
}

func runWithPath(t *testing.T, path string, name string, args ...string) {
	stderr := new(bytes.Buffer)

	cmd := exec.Command(name, args...)
	cmd.Dir = path
	cmd.Stderr = stderr

	if !assert.NoError(t, cmd.Run()) {
		t.Logf("unable to run %s: %s", append([]string{name}, args...), stderr.String())
	}
}
