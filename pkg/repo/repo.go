package repo

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/go-git/go-git/v5"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found: path must be within a git repository")
)

// Repo represents the user's git repository (etok requires that CLI commands
// are run from within a git Repo).
type Repo struct {
	*git.Repository

	// Root module path
	path string
}

// Construct repo obj from a path that exists within a git repo. If path does
// not exist within a git repo, then an error is returned.
func Open(path string) (*Repo, error) {
	gitRepo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			return nil, ErrRepositoryNotFound
		}
		return nil, err
	}

	return &Repo{Repository: gitRepo, path: path}, nil
}

func (r *Repo) Root() string {
	wt, err := r.Worktree()
	if err != nil {
		panic("unable to determine root of git repo: " + err.Error())
	}

	return wt.Filesystem.Root()
}

func (r *Repo) RootModuleRelativePath() (string, error) {
	// Get absolute path to root module
	path, err := filepath.Abs(r.path)
	if err != nil {
		return "", fmt.Errorf("unable to calculate absolute path: %w", err)
	}

	// Get path relative to git repo
	wd, err := filepath.Rel(r.Root(), path)
	if err != nil {
		return "", fmt.Errorf("unable to calculate relative path: %w", err)
	}

	return wd, nil
}

// Retrieve a remote URL for the repo. If there are no remotes, it'll return an
// empty string. If there is more than one remote, it'll return the URL of the
// remote named "origin". If there is no remote named origin it'll return the
// first remote.
func (r *Repo) Url() string {
	remotes, err := r.Remotes()
	if err != nil {
		panic(err.Error())
	}

	if len(remotes) == 0 {
		return ""
	}

	// Return remote named "origin", if it exists
	for _, rem := range remotes {
		if rem.Config().Name == "origin" {
			return normalizeUrl(rem.Config().URLs[0])
		}
	}

	// ...otherwise return first remote
	return normalizeUrl(remotes[0].Config().URLs[0])
}

// Retrieve list of remote URLs for the repo.
func (r *Repo) Urls() []string {
	remotes, err := r.Remotes()
	if err != nil {
		panic(err.Error())
	}

	var ret []string
	if len(remotes) == 0 {
		return ret
	}

	for _, r := range remotes {
		if len(r.Config().URLs) == 0 {
			continue
		}
		ret = append(ret, normalizeUrl(r.Config().URLs[0]))
	}

	return ret
}

var githubNonHttpUrl = regexp.MustCompile(`(?:ssh://)?git@github.com[:/]([\w/\-]+\.git)/?`)

func normalizeUrl(url string) string {
	return githubNonHttpUrl.ReplaceAllString(url, "https://github.com/$1")
}
