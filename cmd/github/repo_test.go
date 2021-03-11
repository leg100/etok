package github

import (
	"strings"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTokenRefresher struct{}

func (r *fakeTokenRefresher) refreshToken() (string, error) {
	return "token123", nil
}

type fakeEvent struct {
	sha, branch string
}

func (e *fakeEvent) GetID() int64          { return 123 }
func (e *fakeEvent) GetHeadBranch() string { return e.branch }
func (e *fakeEvent) GetHeadSHA() string    { return e.sha }

func TestRepo(t *testing.T) {
	path, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")
	dest := testutil.NewTempDir(t).Root()

	r := &repo{
		parentDir:      dest,
		event:          &fakeEvent{sha: sha, branch: "changes"},
		url:            "file://" + path,
		tokenRefresher: &fakeTokenRefresher{},
	}

	err := r.clone()
	require.NoError(t, err)

	out, err := r.runGitCmd("rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(out))

	err = r.reclone()
	require.NoError(t, err)

	out, err = r.runGitCmd("rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(out))

	_, err = r.runGitCmd("commit", "--allow-empty", "-m", "expect this commit to cause the repo to be recloned")
	require.NoError(t, err)

	err = r.ensureCloned()
	require.NoError(t, err)

	// Expect original sha
	out, err = r.runGitCmd("rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, sha, strings.TrimSpace(out))

	r.event.(*fakeEvent).branch = "doesnotexist"
	err = r.reclone()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file://x-access-token:xxxxx@")
}
