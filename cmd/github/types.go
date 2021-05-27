package github

import (
	"context"

	"github.com/google/go-github/v31/github"
)

type githubApp interface {
	handleEvent(event, githubClients) (string, int64, error)
}

type clientGetter interface {
	Get(int64, string) (*github.Client, error)
}

type githubClients struct {
	checks checksClient
	pulls  pullsClient
}

type checksClient interface {
	ListCheckSuitesForRef(ctx context.Context, owner, repo, ref string, opts *github.ListCheckSuiteOptions) (*github.ListCheckSuiteResults, *github.Response, error)
}

type pullsClient interface {
	Get(ctx context.Context, owner, repo string, number int) (*github.PullRequest, *github.Response, error)
}

// A webhook event targeted at a github app
type event interface {
	GetInstallation() *github.Installation
	GetAction() string
}
