package github

import (
	"context"
	"testing"
	"time"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTokenRefresher struct{}

func (r *fakeTokenRefresher) refreshToken() (string, error) {
	return "token123", nil
}

func TestRepoManager(t *testing.T) {
	path, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")
	cloneDir := testutil.NewTempDir(t).Root()

	mgr := newRepoManager(cloneDir)

	// Test cloning
	_, err := mgr.clone(
		"file://"+path,
		"changes",
		sha,
		"bob",
		"myrepo",
		&fakeTokenRefresher{},
	)
	require.NoError(t, err)

	// Test redacting of token
	_, err = mgr.clone(
		"file://"+path,
		"doesnotexist",
		"123abc",
		"bob",
		"myrepo",
		&fakeTokenRefresher{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file://x-access-token:xxxxx@")

	// Test reaper

	// Should be one repo already
	assert.Equal(t, 1, len(mgr.managed))
	// Set artificially low ttl to trigger reaping
	mgr.ttl = time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	// Set artifiically low interval to trigger reaping
	go mgr.reaper(ctx, time.Millisecond)

	// Give reaper more than sufficient time to reap repos
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 0, len(mgr.managed))

	// Clean up go routine
	cancel()
}
