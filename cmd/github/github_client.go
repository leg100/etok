package github

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"

	"github.com/google/go-github/v31/github"
)

type githubOperation interface {
	invoke(*GithubClient) error
}

type GithubClient struct {
	installID int64

	*github.Client

	itr *ghinstallation.Transport

	queue chan githubOperation
}

// NewGithubClient returns a wrapped github client using the 'anonymous' user
func NewAnonymousGithubClient(hostname string) (*GithubClient, error) {
	url := resolveGithubAPIURL(hostname)

	ghClient, err := newGithubClient(url, http.DefaultClient)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing github authentication transport")
	}

	return &GithubClient{
		Client: ghClient,
	}, nil
}

// NewAppGithubClient returns a valid GitHub client.
func NewGithubAppClient(hostname string, appID int64, keyPath string, installID int64) (*GithubClient, error) {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installID, keyPath)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing github authentication transport")
	}

	url := resolveGithubAPIURL(hostname)
	itr.BaseURL = strings.TrimSuffix(url.String(), "/")

	ghClient, err := newGithubClient(url, &http.Client{Transport: itr})
	if err != nil {
		return nil, errors.Wrap(err, "error initializing github authentication transport")
	}

	client := &GithubClient{
		Client: ghClient,
		itr:    itr,
		queue:  make(chan githubOperation, 100),
	}

	go client.processQueue(context.Background())

	return client, err
}

func newGithubClient(url *url.URL, httpClient *http.Client) (*github.Client, error) {
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

	return client, nil
}

func (c *GithubClient) send(op githubOperation) {
	c.queue <- op
}

// Process queue of operations to send to the GH API
func (c *GithubClient) processQueue(ctx context.Context) {
	for {
		select {
		case op := <-c.queue:
			if err := op.invoke(c); err != nil {
				klog.Errorf("failed to invoke github API operation: %s", err.Error())
			}
		case <-ctx.Done():
			klog.Infof("github client: ending process queue: %s", ctx.Err().Error())
			return
		}
	}
}

// refreshToken returns a fresh installation token.
func (c *GithubClient) refreshToken() (string, error) {
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
