package repo

import (
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoUrls(t *testing.T) {
	path := testutil.NewTempDir(t).Root()
	gitRepo, err := git.PlainInit(path, false)
	require.NoError(t, err)

	_, err = gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"git@github.com:leg100/etok.git"},
	})
	_, err = gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "another",
		URLs: []string{"git@github.com:forker/etok.git"},
	})
	require.NoError(t, err)

	repo, err := Open(path)
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Assert that url for remote named origin is returned
	assert.Equal(t, "https://github.com/leg100/etok.git", repo.Url())

	assert.Contains(t, repo.Urls(), "https://github.com/leg100/etok.git")
	assert.Contains(t, repo.Urls(), "https://github.com/forker/etok.git")
}

// Test opening repo with a path in a *subdir* of a repo
func TestRepoOpenInSubDir(t *testing.T) {
	root := testutil.NewTempDir(t).Mkdir("subdir").Root()
	_, err := git.PlainInit(root, false)
	require.NoError(t, err)

	repo, err := Open(filepath.Join(root, "subdir"))
	require.NoError(t, err)
	assert.NotNil(t, repo)
}

func TestRepoError(t *testing.T) {
	_, err := Open(testutil.NewTempDir(t).Root())
	assert.Equal(t, ErrRepositoryNotFound, err)
}

func TestNormalizeUrl(t *testing.T) {
	tests := []string{
		"https://github.com/leg100/etok.git",
		"git@github.com:leg100/etok.git",
		"ssh://git@github.com/leg100/etok.git",
	}

	for _, tt := range tests {
		assert.Equal(t, "https://github.com/leg100/etok.git", normalizeUrl(tt))
	}
}
