package vcs_test

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/leg100/etok/pkg/vcs"
	"github.com/leg100/etok/pkg/vcs/fixtures"
	"github.com/stretchr/testify/require"
)

func TestGithubClient_AppAuthentication(t *testing.T) {
	disableSSLVerification(t)

	testServer, err := fixtures.GithubAppTestServer(t)
	require.NoError(t, err)

	anonCreds := &vcs.GithubAnonymousCredentials{}
	anonClient, err := vcs.NewGithubClient(testServer, anonCreds)
	require.NoError(t, err)
	_, err = anonClient.ExchangeCode("good-code")
	require.NoError(t, err)
}

// disableSSLVerification disables ssl verification for the global http client
// for the duration of the test t
func disableSSLVerification(t *testing.T) {
	orig := http.DefaultTransport.(*http.Transport).TLSClientConfig
	// nolint: gosec
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	t.Cleanup(func() {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = orig
	})
}
