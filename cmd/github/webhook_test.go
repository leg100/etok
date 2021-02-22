package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/github/fixtures"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type githubStatus struct {
	State string
}

func webhookGithubTestServerRouter() (http.Handler, chan string) {
	var counter int
	statuses := make(chan string)
	r := mux.NewRouter()
	r.HandleFunc("/api/v3/repos/Codertocat/Hello-World/statuses/changes", func(w http.ResponseWriter, r *http.Request) {
		var s githubStatus
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		go func() { statuses <- s.State }()

		w.WriteHeader(http.StatusCreated)
	})
	r.HandleFunc("/api/v3/app/installations", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if err := fixtures.ValidateGithubToken(token); err != nil {
			w.WriteHeader(403)
			w.Write([]byte("Invalid token")) // nolint: errcheck
			return
		}

		w.Write([]byte(fixtures.GithubAppInstallationJSON)) // nolint: errcheck
	})
	r.HandleFunc("/api/v3/app/installations/123/access_tokens", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if err := fixtures.ValidateGithubToken(token); err != nil {
			w.WriteHeader(403)
			w.Write([]byte("Invalid token")) // nolint: errcheck
			return
		}

		appToken := fmt.Sprintf(fixtures.GithubAppTokenJSON, counter)
		counter++
		w.Write([]byte(appToken)) // nolint: errcheck
	})
	return r, statuses
}

func webhookGithubTestServer(t *testing.T) (string, chan string) {
	testServer := httptest.NewUnstartedServer(nil)

	// Our fake github router needs the hostname before starting server
	hostname := testServer.Listener.Addr().String()
	router, statuses := webhookGithubTestServerRouter()
	testServer.Config.Handler = router

	testServer.StartTLS()
	return hostname, statuses
}

func TestWebhookServer(t *testing.T) {
	disableSSLVerification(t)

	repo, headSHA := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")

	githubHostname, statuses := webhookGithubTestServer(t)

	ws := testobj.Workspace("default", "default", testobj.WithRepository("file://"+repo), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir"))

	cc := etokclient.NewFakeClientCreator(ws)
	client, err := cc.Create("")
	require.NoError(t, err)

	opts := webhookServerOptions{
		Client:   client,
		cloneDir: testutil.NewTempDir(t).Root(),
		creds: &githubAppCredentials{
			AppID:   1,
			KeyPath: testutil.TempFile(t, "wank", []byte(fixtures.GithubPrivateKey)),
		},
		githubHostname: githubHostname,
		runName:        "run-12345",
	}

	var code int
	opts.runStatus = &v1alpha1.RunStatus{
		Conditions: []metav1.Condition{
			{
				Type:   v1alpha1.RunCompleteCondition,
				Status: metav1.ConditionFalse,
				Reason: v1alpha1.PodRunningReason,
			},
		},
		Phase:    v1alpha1.RunPhaseRunning,
		ExitCode: &code,
	}

	ctx, cancel := context.WithCancel(context.Background())
	errch := make(chan error)
	go func() {
		errch <- opts.run(ctx)
	}()

	req := fixtures.GitHubPullRequestOpenedEvent(t, headSHA, repo)

	w := httptest.NewRecorder()
	opts.webhookEvent(w, req)
	body, _ := ioutil.ReadAll(w.Result().Body)
	if !assert.Equal(t, 200, w.Result().StatusCode) {
		t.Errorf(string(body))
	}
	assert.Equal(t, "progressing...", string(body))

	assert.Equal(t, "pending", <-statuses)
	assert.Equal(t, "success", <-statuses)

	cancel()
	require.NoError(t, <-errch)
}

func initializeRepo(t *testutil.T, seed string) (string, string) {
	repo := t.NewTempDir().Root()

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
