package github

import (
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubClientMap(t *testing.T) {
	clientMap := newGithubClientMap()

	keyPath := testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))

	client, err := clientMap.getClient("app.net", keyPath, 123, 123)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 1, len(clientMap))

	client, err = clientMap.getClient("app.net", keyPath, 123, 456)
	require.NoError(t, err)
	assert.Equal(t, 2, len(clientMap))

	// Test that it retrieves previously created client for install 456 and
	// doesn't create another client
	client2, err := clientMap.getClient("app.net", keyPath, 123, 456)
	require.NoError(t, err)
	assert.Equal(t, 2, len(clientMap))
	assert.Equal(t, client, client2)
}
