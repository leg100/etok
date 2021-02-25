package github

import (
	"strings"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClone(t *testing.T) {
	repo, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")
	dest := testutil.NewTempDir(t).Root()

	// Test clone()
	err := clone("file://"+repo, dest, "changes", "token123")
	require.NoError(t, err)

	out, err := runGitCmd(dest, "rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(out))

	// Test reclone()
	err = reclone("file://"+repo, dest, "changes", "token123")
	require.NoError(t, err)

	out, err = runGitCmd(dest, "rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(out))
}

func TestEnsureCloned(t *testing.T) {
	repo, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")
	dest := testutil.NewTempDir(t).Root()

	err := clone("file://"+repo, dest, "changes", "token123")
	require.NoError(t, err)

	_, err = runGitCmd(dest, "commit", "--allow-empty", "-m", "expect this commit to cause the repo to be recloned")
	require.NoError(t, err)

	err = ensureCloned("file://"+repo, dest, "changes", sha, "token123")
	require.NoError(t, err)

	// Expect original sha
	out, err := runGitCmd(dest, "rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(out))
}

func TestRedactedToken(t *testing.T) {
	repo, _ := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")
	dest := testutil.NewTempDir(t).Root()

	err := clone("file://"+repo, dest, "doesnotexist", "token123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file://x-access-token:xxxxx@")
}
