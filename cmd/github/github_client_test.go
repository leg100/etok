package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestGithubClient(t *testing.T) {
	disableSSLVerification(t)

	var counter int
	testServer := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.RequestURI {
			case "/api/v3/app/installations/123/access_tokens":
				appToken := fmt.Sprintf(fixtures.GithubAppTokenJSON, counter)
				counter++
				w.Write([]byte(appToken)) // nolint: errcheck
			default:
				t.Errorf("got unexpected request at %q", r.RequestURI)
				http.Error(w, "not found", http.StatusNotFound)
			}
		}))

	testServerURL, err := url.Parse(testServer.URL)
	require.NoError(t, err)

	_, err = NewAnonymousGithubClient(testServerURL.Host)
	require.NoError(t, err)

	keyPath := testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))

	client, err := NewGithubAppClient(testServerURL.Host, 123, keyPath, 123)
	require.NoError(t, err)

	_, err = client.refreshToken()
	require.NoError(t, err)
}
