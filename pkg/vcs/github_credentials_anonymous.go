package vcs

import "net/http"

// GithubAnonymousCredentials expose no credentials.
type GithubAnonymousCredentials struct{}

// Client returns a client with no credentials.
func (c *GithubAnonymousCredentials) Client() (*http.Client, error) {
	tr := http.DefaultTransport
	return &http.Client{Transport: tr}, nil
}

// GetUser returns the username for these credentials.
func (c *GithubAnonymousCredentials) GetUser() string {
	return "anonymous"
}

// GetToken returns an empty token.
func (c *GithubAnonymousCredentials) GetToken() (string, error) {
	return "", nil
}
