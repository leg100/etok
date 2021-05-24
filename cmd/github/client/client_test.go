package client

import (
	"net/url"
	"testing"

	"github.com/leg100/etok/cmd/github/client/fixtures"
	"github.com/stretchr/testify/assert"
)

func TestClient(t *testing.T) {
	client, err := NewAnonymous("github.com")
	assert.NoError(t, err)
	assert.Equal(t, &url.URL{Scheme: "https", Host: "api.github.com", Path: "/"}, client.BaseURL)

	client, err = NewAnonymous("my-github-enterprise.com")
	assert.NoError(t, err)
	assert.Equal(t, &url.URL{Scheme: "https", Host: "my-github-enterprise.com", Path: "/api/v3/"}, client.BaseURL)

	transport, err := newTransport("github.com", 123, 456, []byte(fixtures.GithubPrivateKey))
	assert.NoError(t, err)
	assert.Equal(t, "https://api.github.com", transport.BaseURL)

	transport, err = newTransport("my-github-enterprise.com", 123, 456, []byte(fixtures.GithubPrivateKey))
	assert.NoError(t, err)
	assert.Equal(t, "https://my-github-enterprise.com/api/v3", transport.BaseURL)
}
