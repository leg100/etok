package github

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApp struct{}

func (a *fakeApp) handleEvent(_ event, _ githubClients) (string, int64, error) {
	return "", 0, nil
}

func TestWebhookServer(t *testing.T) {
	server := webhookServer{
		app:    &fakeApp{},
		getter: &fakeClientGetter{},
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

	requestJSON, _ := os.ReadFile("fixtures/newCheckSuiteEvent.json")

	url := fmt.Sprintf("http://localhost:%d/events", server.port)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(requestJSON)))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(githubHeader, "check_suite")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	cancel()
	require.NoError(t, <-errch)
}
