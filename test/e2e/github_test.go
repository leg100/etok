package e2e

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-github/v31/github"
	expect "github.com/google/goexpect"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
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

	installID, err := strconv.ParseInt(os.Getenv("GITHUB_E2E_INSTALL_ID"), 10, 64)
	require.NoError(t, err)

	// Setup github client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	gclient := github.NewClient(tc)

	// Path to cloned repo
	path := testutil.NewTempDir(t).Root()

	t.Run("remove all checksuites", func(t *testing.T) {
		require.NoError(t, rclient.DeleteAllOf(context.Background(), &v1alpha1.CheckSuite{}))
	})

	t.Run("delete existing branch", func(t *testing.T) {
		// This deletes the PR too
		gclient.Git.DeleteRef(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), "heads/e2e")
	})

	t.Run("recreate namespace", func(t *testing.T) {
		// (Re-)create dedicated namespace for e2e test
		deleteNamespace(t, namespace)
		createNamespace(t, namespace)
	})

	t.Run("clone", func(t *testing.T) {
		require.NoError(t, exec.Command("git", "clone", os.Getenv("GITHUB_E2E_REPO_URL"), path).Run())
	})

	// Now we have a cloned repo we can create some workspaces, which'll
	// automatically 'belong' to the repo
	t.Run("create workspace", func(t *testing.T) {
		require.NoError(t, step(t, name,
			[]string{buildPath, "workspace", "new", "foo",
				"--namespace", namespace,
				"--path", path,
				"--context", *kubectx,
				"--ephemeral",
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

	t.Run("create pull request", func(t *testing.T) {
		_, _, err := gclient.PullRequests.Create(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), &github.NewPullRequest{
			Title: github.String("e2e"),
			Head:  github.String("e2e"),
			Base:  github.String("master"),
		})
		require.NoError(t, err)
	})

	t.Run("await completion of check runs", func(t *testing.T) {
		err := wait.Poll(time.Second, 10*time.Second, func() (bool, error) {
			results, _, err := gclient.Checks.ListCheckRunsForRef(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), "e2e", nil)
			if err != nil {
				return false, err
			}

			if len(results.CheckRuns) == 0 {
				return false, nil
			}

			check := results.CheckRuns[0]

			t.Logf("check run update: id=%d, status=%s", check.GetID(), check.GetStatus())

			if check.GetStatus() != "completed" {
				return false, nil
			}

			require.Equal(t, "success", check.GetConclusion())
			return true, nil
		})
		require.NoError(t, err)
	})

	// The only way to trigger an apply is to construct an event and send it to
	// our webhook server.
	t.Run("trigger apply", func(t *testing.T) {
		results, _, err := gclient.Checks.ListCheckRunsForRef(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), "e2e", nil)
		require.NoError(t, err)
		require.Equal(t, 1, results.GetTotal())

		event := github.CheckRunEvent{
			Action:   github.String("requested_action"),
			CheckRun: results.CheckRuns[0],
			Installation: &github.Installation{
				ID: &installID,
			},
			Repo: &github.Repository{
				CloneURL: github.String(fmt.Sprintf("https://github.com/%s/%s.git", os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"))),
				Owner: &github.User{
					Login: github.String(os.Getenv("GITHUB_E2E_REPO_OWNER")),
				},
				Name: github.String(os.Getenv("GITHUB_E2E_REPO_NAME")),
			},
			RequestedAction: &github.RequestedAction{
				Identifier: "apply",
			},
		}

		sendEvent(t, "check_run", event)
	})

	t.Run("await completion of apply", func(t *testing.T) {
		err := wait.Poll(time.Second, 10*time.Second, func() (bool, error) {
			results, _, err := gclient.Checks.ListCheckRunsForRef(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), "e2e", nil)
			if err != nil {
				return false, err
			}
			if len(results.CheckRuns) == 0 {
				return false, nil
			}
			check := results.CheckRuns[0]

			t.Logf("check run update: id=%d, status=%s", check.GetID(), check.GetStatus())

			if check.GetStatus() != "completed" {
				return false, nil
			}

			require.Equal(t, "success", check.GetConclusion())
			return true, nil
		})
		require.NoError(t, err)
	})

	// The only way to trigger a rerequest of a check suite is to construct an
	// event and send it to our webhook server.
	t.Run("trigger check suite rerequest", func(t *testing.T) {
		// Lookup corresponding Check Suite ID in GH API
		results, _, err := gclient.Checks.ListCheckSuitesForRef(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), "e2e", nil)
		require.NoError(t, err)
		require.Equal(t, 1, *results.Total)

		event := github.CheckSuiteEvent{
			Action:     github.String("rerequested"),
			CheckSuite: results.CheckSuites[0],
			Installation: &github.Installation{
				ID: &installID,
			},
			Repo: &github.Repository{
				CloneURL: github.String(fmt.Sprintf("https://github.com/%s/%s.git", os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"))),
				Owner: &github.User{
					Login: github.String(os.Getenv("GITHUB_E2E_REPO_OWNER")),
				},
				Name: github.String(os.Getenv("GITHUB_E2E_REPO_NAME")),
			},
		}

		sendEvent(t, "check_suite", event)
	})

	t.Run("await completion of rerequested check run", func(t *testing.T) {
		err := wait.Poll(time.Second, 10*time.Second, func() (bool, error) {
			results, _, err := gclient.Checks.ListCheckRunsForRef(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), "e2e", nil)
			if err != nil {
				return false, err
			}

			// Should have at least one check run from before the rerequest of
			// the check suite
			require.GreaterOrEqual(t, len(results.CheckRuns), 1)

			if len(results.CheckRuns) < 2 {
				return false, nil
			}

			for _, check := range results.CheckRuns {
				check.CheckSuite, _, err = gclient.Checks.GetCheckSuite(context.Background(), os.Getenv("GITHUB_E2E_REPO_OWNER"), os.Getenv("GITHUB_E2E_REPO_NAME"), check.CheckSuite.GetID())
				require.NoError(t, err)

				t.Logf("check run update: id=%d, status=%s", check.GetID(), check.GetStatus())

				if check.GetStatus() != "completed" {
					return false, nil
				}

				require.Equal(t, "success", check.GetConclusion())
			}
			return true, nil
		})
		require.NoError(t, err)
	})

}

func sendEvent(t *testing.T, name string, event interface{}) {
	// Encode event to a json payload
	buf := new(bytes.Buffer)
	require.NoError(t, json.NewEncoder(buf).Encode(event))

	// Generate HMAC of payload using webhook secret
	hash := hmac.New(sha1.New, []byte(os.Getenv("GITHUB_E2E_WEBHOOK_SECRET")))
	hash.Write(buf.Bytes())

	// Construct HTTP request
	req, err := http.NewRequest("POST", os.Getenv("GITHUB_E2E_URL")+"/events", buf)
	require.NoError(t, err)
	req.Header.Set("X-GitHub-Event", name)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature", "sha1="+hex.EncodeToString(hash.Sum(nil)))

	// Send event
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	if !assert.Equal(t, 200, resp.StatusCode) {
		errmsg, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)
		t.Logf("received response: %s", string(errmsg))
	}
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
