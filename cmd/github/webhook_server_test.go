package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/gorilla/mux"
	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func webhookGithubTestServerRouter() (http.Handler, chan interface{}) {
	checkRunObjs := make(chan interface{}, 100)
	var counter int
	r := mux.NewRouter()

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
	r.HandleFunc("/api/v3/app/installations/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.Replace(r.Header.Get("Authorization"), "Bearer ", "", 1)
		if err := fixtures.ValidateGithubToken(token); err != nil {
			w.WriteHeader(403)
			w.Write([]byte("Invalid token")) // nolint: errcheck
			return
		}

		w.Write([]byte(fixtures.GithubAppInstallationJSON)) // nolint: errcheck
	})

	checkRuns := r.PathPrefix("/repos/bob/myrepo/check-runs").Subrouter()
	checkRuns.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		opts := github.CreateCheckRunOptions{}
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Unable to decode JSON into a create check runs options object"))
			return
		}
		checkRunObjs <- opts
	}).Methods("POST")

	return r, checkRunObjs
}

func webhookGithubTestServer(t *testing.T, h http.Handler) string {
	testServer := httptest.NewUnstartedServer(h)

	// Our fake github router needs the hostname before starting server
	hostname := testServer.Listener.Addr().String()

	testServer.StartTLS()
	return hostname
}

type fakeApp struct{}

func (a *fakeApp) handleEvent(client *GithubClient, ev interface{}) error {
	return nil
}

func TestWebhookServer(t *testing.T) {
	disableSSLVerification(t)

	hdlr, _ := webhookGithubTestServerRouter()
	githubHostname := webhookGithubTestServer(t, hdlr)

	server := newWebhookServer(&fakeApp{})

	server.appID = 1
	server.keyPath = testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))
	server.githubHostname = githubHostname

	ctx, cancel := context.WithCancel(context.Background())
	errch := make(chan error)
	go func() {
		errch <- server.run(ctx)
	}()

	// Wait for dynamic port to be assigned
	for {
		if server.port != 0 {
			break
		}
	}

	req := fixtures.GitHubNewCheckSuiteEvent(t, server.port, "abc123", "https://foo.bar.git")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	cancel()
	require.NoError(t, <-errch)
}
