package github

import (
	"context"

	"github.com/google/go-github/v31/github"
)

type githubApp interface {
	handleEvent(interface{}, string, checksClient) (string, int64, error)
}

type clientGetter interface {
	Get(int64, string) (*github.Client, error)
}

type checksClient interface {
	ListCheckSuitesForRef(ctx context.Context, owner, repo, ref string, opts *github.ListCheckSuiteOptions) (*github.ListCheckSuiteResults, *github.Response, error)
}

type installEvent interface {
	GetInstallation() *github.Installation
}

type actionEvent interface {
	GetAction() string
}
