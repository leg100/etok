package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApp struct{}

func (a *fakeApp) handleEvent(client *GithubClient, ev interface{}) error {
	return nil
}

func TestWebhookServer(t *testing.T) {
	disableSSLVerification(t)

	// Start a mock github API
	githubHostname, _ := fixtures.GithubServer(t)

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

	// Setup mock repo
	path, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")

	req := fixtures.GitHubNewCheckSuiteEvent(t, server.port, sha, path)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	cancel()
	require.NoError(t, <-errch)
}
