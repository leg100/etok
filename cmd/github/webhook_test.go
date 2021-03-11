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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/github/fixtures"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func webhookGithubTestServerRouter(headSHA, repo string) (http.Handler, chan github.CheckRun) {
	var counter int
	checkRuns := make(chan github.CheckRun, 100)
	r := mux.NewRouter()
	r.HandleFunc("/api/v3/repos/Codertocat/Hello-World/check-runs", func(w http.ResponseWriter, r *http.Request) {
		var cr github.CheckRun
		if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		w.WriteHeader(http.StatusCreated)

		// Respond back with check run with faked ID
		var id int64 = 666
		cr.ID = &id
		cr.CheckSuite = fixtures.GithubCheckSuite(headSHA, repo)
		json.NewEncoder(w).Encode(cr)

		checkRuns <- cr
	})
	r.HandleFunc("/api/v3/repos/Codertocat/Hello-World/check-runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		var cr github.CheckRun
		if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		w.WriteHeader(http.StatusOK)

		checkRuns <- cr
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
	return r, checkRuns
}

func webhookGithubTestServer(t *testing.T, headSHA, repo string) (string, chan github.CheckRun) {
	testServer := httptest.NewUnstartedServer(nil)

	// Our fake github router needs the hostname before starting server
	hostname := testServer.Listener.Addr().String()
	router, statuses := webhookGithubTestServerRouter(headSHA, repo)
	testServer.Config.Handler = router

	testServer.StartTLS()
	return hostname, statuses
}

func TestWebhookServer(t *testing.T) {
	disableSSLVerification(t)

	repo, headSHA := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")

	githubHostname, checkRuns := webhookGithubTestServer(t, headSHA, repo)

	ws := testobj.Workspace("default", "default", testobj.WithRepository("file://"+repo), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir"))

	cc := etokclient.NewFakeClientCreator(ws)
	client, err := cc.Create("")
	require.NoError(t, err)

	server := newWebhookServer()
	// k8s client
	server.Client = client

	server.cloneDir = testutil.NewTempDir(t).Root()
	server.appID = 1
	server.keyPath = testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))
	server.githubHostname = githubHostname

	var code int
	server.checkRunOptions = checkRunOptions{
		runStatus: &v1alpha1.RunStatus{
			Conditions: []metav1.Condition{
				{
					Type:   v1alpha1.RunCompleteCondition,
					Status: metav1.ConditionFalse,
					Reason: v1alpha1.PodRunningReason,
				},
			},
			Phase:    v1alpha1.RunPhaseRunning,
			ExitCode: &code,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	errch := make(chan error)
	go func() {
		errch <- server.run(ctx)
	}()

	req := fixtures.GitHubNewCheckSuiteEvent(t, headSHA, repo)

	w := httptest.NewRecorder()
	go server.eventHandler(w, req)

	body, _ := ioutil.ReadAll(w.Result().Body)
	if !assert.Equal(t, 200, w.Result().StatusCode) {
		t.Errorf(string(body))
	}

	// Expect check run creation
	create := <-checkRuns
	assert.Equal(t, "in_progress", *create.Status)

	// Expect check run update
	update := <-checkRuns
	assert.Equal(t, "completed", *update.Status)
	assert.Equal(t, "success", *update.Conclusion)
	assert.Equal(t, "fake logs", *update.Output.Text)

	// Expect one run to have been created
	runlist, err := client.RunsClient("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, len(runlist.Items))

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
