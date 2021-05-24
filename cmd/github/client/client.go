package client

import (
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v31/github"
)

// Return a bog-standard unauthenticated vanilla github client
func NewAnonymous(hostname string) (*github.Client, error) {
	return newClient(hostname, http.DefaultClient)
}

// NewInstall returns a github client that authenticates as an installation:
//
// https://docs.github.com/en/developers/apps/building-github-apps/authenticating-with-github-apps#authenticating-as-an-installation
//
func NewInstall(hostname string, appID int64, installID int64, key []byte) (*github.Client, error) {
	transport, err := newTransport(hostname, appID, installID, key)
	if err != nil {
		return nil, err
	}

	return newClient(hostname, &http.Client{Transport: transport})
}

// Returns an http.RoundTripper implementation that provides Github Apps
// authentication as an installation
func newTransport(hostname string, appID int64, installID int64, key []byte) (*ghinstallation.Transport, error) {
	transport, err := ghinstallation.New(http.DefaultTransport, appID, installID, key)
	if err != nil {
		return nil, err
	}

	if isEnterprise(hostname) {
		transport.BaseURL = enterpriseURL(hostname)
	}
	return transport, nil
}

// Create a new github client for either github.com or enterprise
func newClient(hostname string, httpClient *http.Client) (*github.Client, error) {
	if isEnterprise(hostname) {
		url := enterpriseURL(hostname)
		return github.NewEnterpriseClient(url, url, httpClient)
	}
	return github.NewClient(httpClient), nil
}

// Return a github enterprise URL from a hostname
func enterpriseURL(hostname string) string {
	return (&url.URL{Scheme: "https", Host: hostname, Path: "/api/v3"}).String()
}

func isEnterprise(hostname string) bool {
	return hostname != "github.com"
}
