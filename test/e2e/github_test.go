package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-github/v31/github"
	expect "github.com/google/goexpect"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/util/wait"
)

// E2E test of github webhook
func TestGithub(t *testing.T) {
	// Only run github tests on clusters exposed to internet, or when explicitly
	// asked to.
	if *kubectx == "kind-kind" && os.Getenv("GITHUB_E2E_TEST") != "true" {
		t.SkipNow()
	}

	t.Parallel()

	name := "github"
	namespace := "e2e-github"

	// Setup github client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(tc)

	// Path to cloned repo
	path := testutil.NewTempDir(t).Root()

	t.Run("create namespace", func(t *testing.T) {
		// (Re-)create dedicated namespace for e2e test
		deleteNamespace(t, namespace)
		createNamespace(t, namespace)
	})

	t.Run("clone", func(t *testing.T) {
		require.NoError(t, exec.Command("git", "clone", os.Getenv("REPO_URL"), path).Run())
	})

	// Now we have a cloned repo we can create a workspace, which'll
	// automatically 'belong' to the repo
	t.Run("create workspace", func(t *testing.T) {
		require.NoError(t, step(t, name,
			[]string{buildPath, "workspace", "new", "foo",
				"--namespace", namespace,
				"--path", path,
				"--context", *kubectx,
				"--ephemeral",
				"--variables", "suffix=bar",
			},
			[]expect.Batcher{
				&expect.BExp{R: fmt.Sprintf("Created workspace %s/foo", namespace)},
			}))
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

	// Ensure check runs have been created - list all runs for HEAD of master
	var checkRuns []*github.CheckRun
	err := wait.Poll(time.Second, 10*time.Second, func() (bool, error) {
		results, _, err := client.Checks.ListCheckRunsForRef(context.Background(), "leg100", "etok-e2e", "e2e", nil)
		if err != nil {
			return false, err
		}
		if len(results.CheckRuns) > 0 {
			checkRuns = results.CheckRuns
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, len(checkRuns))

	// Ensure run resource has been created
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
