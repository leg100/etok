package workspace

import (
	"regexp"

	"github.com/go-git/go-git/v5"
)

type repo struct {
	*git.Repository
}

func openRepo(path string) (*repo, error) {
	gitRepo, err := git.PlainOpenWithOptions(path, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, err
	}

	return &repo{gitRepo}, nil
}

// Retrieve a remote URL for the repo. If there are no remotes, it'll return an
// empty string. If there is more than one remote, it'll return the URL of the
// first remote.
func (r *repo) url() string {
	remotes, err := r.Remotes()
	if err != nil {
		panic(err.Error())
	}

	if len(remotes) == 0 {
		return ""
	}

	if len(remotes[0].Config().URLs) == 0 {
		return ""
	}

	return remotes[0].Config().URLs[0]
}

// Retrieve list of remote URLs for the repo.
func (r *repo) urls() []string {
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
		ret = append(ret, r.Config().URLs[0])
	}

	return ret
}

var githubNonHttpUrl = regexp.MustCompile(`(?:ssh://)?git@github.com[:/]([\w/\-]+\.git)/?`)

func normalizeUrl(url string) string {
	return githubNonHttpUrl.ReplaceAllString(url, "https://github.com/$1")
}
