package github

import (
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubClientManager(t *testing.T) {
	keyPath := testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))

	mgr, err := newGithubClientManager("app.net", keyPath, 123)

	client, err := mgr.getOrCreate(123)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 1, len(mgr.clients))

	client, err = mgr.getOrCreate(456)
	require.NoError(t, err)
	assert.Equal(t, 2, len(mgr.clients))

	// Test that it retrieves previously created client for install 456 and
	// doesn't create another client
	client2, err := mgr.getOrCreate(456)
	require.NoError(t, err)
	assert.Equal(t, 2, len(mgr.clients))
	assert.Equal(t, client, client2)
}
