package vcs

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pkg/errors"

	"github.com/google/go-github/v31/github"
)

// NewGithubClient returns a valid GitHub client.
func NewGithubClient(hostname string, credentials GithubCredentials) (*github.Client, error) {
	transport, err := credentials.Client()
	if err != nil {
		return nil, errors.Wrap(err, "error initializing github authentication transport")
	}

	var client *github.Client
	if hostname == "github.com" {
		client = github.NewClient(transport)
	} else {
		apiURL := resolveGithubAPIURL(hostname)
		client, err = github.NewEnterpriseClient(apiURL.String(), apiURL.String(), transport)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func resolveGithubAPIURL(hostname string) *url.URL {
	// If we're using github.com then we don't need to do any additional configuration
	// for the client. It we're using Github Enterprise, then we need to manually
	// set the base url for the API.
	baseURL := &url.URL{
		Scheme: "https",
		Host:   "api.github.com",
		Path:   "/",
	}

	if hostname != "github.com" {
		baseURL.Host = hostname
		baseURL.Path = "/api/v3/"
	}

	return baseURL
}

// UpdateStatus updates the status badge on the pull request.  See
// https://github.com/blog/1227-commit-status-api.
func UpdateStatus(client *github.Client, state, description, cmd string, pr *github.PullRequestEvent) error {
	status := &github.RepoStatus{
		State:       github.String(state),
		Description: github.String(description),
		Context:     github.String(fmt.Sprintf("etok/%s", cmd)),
	}
	_, _, err := client.Repositories.CreateStatus(context.Background(), *pr.Repo.Owner.Login, *pr.Repo.Name, *pr.PullRequest.Head.Ref, status)
	return err
}
