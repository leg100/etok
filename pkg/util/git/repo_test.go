package git

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGitRepoPath(t *testing.T) {
	testutil.Run(t, "inside git repo", func(t *testutil.T) {
		// Setup git repo with directory inside it
		repo := t.NewTempDir()
		repo.Mkdir(".git")
		subdir := repo.Mkdir("subdir").Root()

		got, err := GetRepoRoot(subdir)
		require.NoError(t, err)

		assert.Equal(t, repo.Root(), got)
	})

	testutil.Run(t, "outside git repo", func(t *testutil.T) {
		// Setup directory not within a git repo
		dir := t.NewTempDir().Root()

		_, err := GetRepoRoot(dir)
		require.Equal(t, ErrNotRepo, err)
	})
}
