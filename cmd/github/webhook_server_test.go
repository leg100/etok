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

func (a *fakeApp) handleEvent(ev interface{}) error {
	return nil
}

type fakeGithubClientManager struct{}

func (m *fakeGithubClientManager) getOrCreate(installID int64) (*GithubClient, error) {
	return &GithubClient{}, nil
}

func TestWebhookServer(t *testing.T) {
	server := webhookServer{
		app: &fakeApp{},
	}

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
