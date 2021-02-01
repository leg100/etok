package vcs

import (
	"context"
	"net/url"

	"github.com/pkg/errors"

	"github.com/google/go-github/v31/github"
)

// GithubClient is used to perform GitHub actions.
type GithubClient struct {
	user   string
	client *github.Client
	ctx    context.Context
}

// GithubAppTemporarySecrets holds app credentials obtained from github after creation.
type GithubAppTemporarySecrets struct {
	// ID is the app id.
	ID int64
	// Key is the app's PEM-encoded key.
	Key string
	// Name is the app name.
	Name string
	// WebhookSecret is the generated webhook secret for this app.
	WebhookSecret string
	// URL is a link to the app, like https://github.com/apps/octoapp.
	URL string
}

// NewGithubClient returns a valid GitHub client.
func NewGithubClient(hostname string, credentials GithubCredentials) (*GithubClient, error) {
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

	return &GithubClient{
		user:   credentials.GetUser(),
		client: client,
		ctx:    context.Background(),
	}, nil
}

// ExchangeCode returns a newly created app's info
func (g *GithubClient) ExchangeCode(code string) (*GithubAppTemporarySecrets, error) {
	ctx := context.Background()
	cfg, _, err := g.client.Apps.CompleteAppManifest(ctx, code)
	data := &GithubAppTemporarySecrets{
		ID:            cfg.GetID(),
		Key:           cfg.GetPEM(),
		WebhookSecret: cfg.GetWebhookSecret(),
		Name:          cfg.GetName(),
		URL:           cfg.GetHTMLURL(),
	}

	return data, err
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
