package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/leg100/etok/cmd/github/client/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager(t *testing.T) {
	t.Run("access token", func(t *testing.T) {
		testutil.DisableSSLVerification(t)

		server := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.RequestURI {
				case "/api/v3/app/installations/123/access_tokens":
					w.Write([]byte(fixtures.GithubAppTokenJSON))
				default:
					t.Errorf("got unexpected request at %q", r.RequestURI)
					http.Error(w, "not found", http.StatusNotFound)
				}
			}))

		url, err := url.Parse(server.URL)
		require.NoError(t, err)
		key := testutil.TempFile(t, "", []byte(fixtures.GithubPrivateKey))

		mgr, err := NewManager(key, 123)
		require.NoError(t, err)

		token, err := mgr.Token(context.Background(), 123, url.Host)
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("cache", func(t *testing.T) {
		key := testutil.TempFile(t, "", []byte(fixtures.GithubPrivateKey))

		mgr, err := NewManager(key, 123)
		require.NoError(t, err)

		client, err := mgr.get(456, "github.com")
		require.NoError(t, err)

		cached, ok := mgr.clients[456]
		if assert.True(t, ok) {
			assert.Equal(t, client, cached)
		}
	})
}
