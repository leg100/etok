package github

import (
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubClientManager(t *testing.T) {
	clientManager := newGithubClientManager()

	keyPath := testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))

	client, err := clientManager.getOrCreate("app.net", keyPath, 123, 123)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 1, len(clientManager))

	client, err = clientManager.getOrCreate("app.net", keyPath, 123, 456)
	require.NoError(t, err)
	assert.Equal(t, 2, len(clientManager))

	// Test that it retrieves previously created client for install 456 and
	// doesn't create another client
	client2, err := clientManager.getOrCreate("app.net", keyPath, 123, 456)
	require.NoError(t, err)
	assert.Equal(t, 2, len(clientManager))
	assert.Equal(t, client, client2)
}
