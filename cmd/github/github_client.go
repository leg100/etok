package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/pkg/errors"

	"github.com/google/go-github/v31/github"
)

type githubClient struct {
	*github.Client

	itr *ghinstallation.Transport
}

// githubAppCredentials contains credentials for a github app installation.
type githubAppCredentials struct {
	AppID          int64
	KeyPath        string
	InstallationID int64
}

// newGithubClient returns a valid GitHub client. If credentials is nil then an
// 'anonymous' client will be returned.
func newGithubClient(hostname string, creds *githubAppCredentials) (*githubClient, error) {
	var itr *ghinstallation.Transport
	httpClient := http.DefaultClient

	url := resolveGithubAPIURL(hostname)

	if creds != nil {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, creds.AppID, creds.InstallationID, creds.KeyPath)
		if err != nil {
			return nil, errors.Wrap(err, "error initializing github authentication transport")
		}

		itr.BaseURL = strings.TrimSuffix(url.String(), "/")

		httpClient = &http.Client{Transport: itr}
	}

	var client *github.Client
	if url.Host == "api.github.com" {
		client = github.NewClient(httpClient)
	} else {
		var err error
		client, err = github.NewEnterpriseClient(url.String(), url.String(), httpClient)
		if err != nil {
			return nil, err
		}
	}

	return &githubClient{
		Client: client,
		itr:    itr,
	}, nil
}

// refreshToken returns a fresh installation token.
func (c *githubClient) refreshToken() (string, error) {
	if c.itr != nil {
		return c.itr.Token(context.Background())
	}
	return "", errors.New("No credentials found with which to generate installation token")
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

// updateStatus updates the status badge on the pull request.  See
// https://github.com/blog/1227-commit-status-api.
func updateStatus(client *githubClient, state, description, cmd string, pr *github.PullRequestEvent) error {
	status := &github.RepoStatus{
		State:       github.String(state),
		Description: github.String(description),
		Context:     github.String(fmt.Sprintf("etok/%s", cmd)),
	}
	_, _, err := client.Repositories.CreateStatus(context.Background(), *pr.Repo.Owner.Login, *pr.Repo.Name, *pr.PullRequest.Head.Ref, status)
	return err
}
