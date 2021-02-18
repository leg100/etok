package vcs

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/pkg/errors"
)

// GithubAppCredentials implements GithubCredentials for github app installation
// token flow.
type GithubAppCredentials struct {
	AppID          int64
	KeyPath        string
	Hostname       string
	InstallationID int64
	apiURL         *url.URL
	tr             *ghinstallation.Transport
}

// Client returns a github app installation client.
func (c *GithubAppCredentials) Client() (*http.Client, error) {
	itr, err := c.transport()
	if err != nil {
		return nil, err
	}
	return &http.Client{Transport: itr}, nil
}

// GetUser returns the username for these credentials.
func (c *GithubAppCredentials) GetUser() string {
	return ""
}

// GetToken returns a fresh installation token.
func (c *GithubAppCredentials) GetToken() (string, error) {
	tr, err := c.transport()
	if err != nil {
		return "", errors.Wrap(err, "transport failed")
	}

	return tr.Token(context.Background())
}

func (c *GithubAppCredentials) transport() (*ghinstallation.Transport, error) {
	if c.tr != nil {
		return c.tr, nil
	}

	tr := http.DefaultTransport
	itr, err := ghinstallation.NewKeyFromFile(tr, c.AppID, c.InstallationID, c.KeyPath)
	if err == nil {
		apiURL := c.getAPIURL()
		itr.BaseURL = strings.TrimSuffix(apiURL.String(), "/")
		c.tr = itr
	}
	return itr, err
}

func (c *GithubAppCredentials) getAPIURL() *url.URL {
	if c.apiURL != nil {
		return c.apiURL
	}

	c.apiURL = resolveGithubAPIURL(c.Hostname)
	return c.apiURL
}
