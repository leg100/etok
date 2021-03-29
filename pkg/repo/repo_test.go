package repo

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepo(t *testing.T) {
	tmp := testutil.NewTempDir(t)
	gitRepo, err := git.PlainInit(tmp.Root(), false)
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

	repo, err := Open(tmp.Root())
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Assert that url for remote named origin is returned
	assert.Equal(t, "https://github.com/leg100/etok.git", repo.url())

	assert.Equal(t, []string{
		"https://github.com/leg100/etok.git",
		"https://github.com/forker/etok.git",
	}, repo.urls())
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
