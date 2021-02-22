package github

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGithubClientUpdateStatus(t *testing.T) {
	disableSSLVerification(t)

	var counter int
	testServer := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.RequestURI {
			case "/api/v3/repos/Codertocat/Hello-World/statuses/changes":
				body, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				exp := fmt.Sprintf(`{"state":"%s","description":"in progress...","context":"etok/plan"}%s`, "in progress", "\n")
				assert.Equal(t, exp, string(body))
				defer r.Body.Close() // nolint: errcheck
				w.WriteHeader(http.StatusOK)
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

	creds := githubAppCredentials{
		AppID:          123,
		KeyPath:        testutil.TempFile(t, "wank", []byte(fixtures.GithubPrivateKey)),
		InstallationID: 123,
	}

	client, err := NewGithubClient(testServerURL.Host, &creds)
	require.NoError(t, err)

	event, err := validateAndParse(fixtures.GitHubPullRequestOpenedEvent(t, "123", "myrepo"), nil)
	require.NoError(t, err)

	err = updateStatus(client, "in progress", "in progress...", "plan", event.(*github.PullRequestEvent))
	require.NoError(t, err)
}
