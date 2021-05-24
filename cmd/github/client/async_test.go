package client

import (
	"context"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/cmd/github/client/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeInvokable struct {
	invoked chan struct{}
}

func (i *fakeInvokable) Invoke(client *github.Client) error {
	i.invoked <- struct{}{}
	return nil
}

func TestAsync(t *testing.T) {
	client, err := newAsync("github.com", 123, 456, []byte(fixtures.GithubPrivateKey))
	require.NoError(t, err)

	// Process jobs in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.process(ctx)

	job := &fakeInvokable{
		invoked: make(chan struct{}),
	}

	// Send and verify job is invoked
	client.send(job)
	assert.NotNil(t, <-job.invoked)
}
