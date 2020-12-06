package git

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrNotRepo = errors.New("path not within a git repository")

// getGitRepoRoot finds the path to the root of the git repository in which path
// is within
func GetRepoRoot(path string) (string, error) {
	gitdir, err := os.Stat(filepath.Join(path, ".git"))
	if os.IsNotExist(err) || !gitdir.IsDir() {
		parent := filepath.Dir(path)
		if parent == path {
			// Reached root (or c:\ ?) without finding .git
			return "", ErrNotRepo
		}
		return GetRepoRoot(parent)
	}
	if err != nil {
		// Some error other than not found
		return "", err
	}
	return path, nil
}
