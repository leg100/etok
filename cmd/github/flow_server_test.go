package github

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/leg100/etok/pkg/static"
	"github.com/leg100/etok/pkg/vcs/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unrolled/render"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGithubAppController(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, err := fixtures.GithubAppTestServer(t)
	require.NoError(t, err)

	client := fake.NewSimpleClientset().CoreV1().Secrets("fake")

	controller := &appController{
		secretMaker: secretMaker{
			client: client,
			name:   secretName,
		},
		githubHostname: githubHostname,
		name:           "etok-app",
		port:           12345,
		webhookUrl:     &url.URL{Scheme: "https", Host: "webhook.etok.dev"},
		Render: render.New(
			render.Options{
				Asset:      static.Asset,
				AssetNames: static.AssetNames,
				Directory:  "static/templates",
			},
		),
	}

	req, err := http.NewRequest("GET", "/github-app/setup", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	controller.newApp(w, req)
	assert.Equal(t, 200, w.Result().StatusCode)

	// Make request for exchange code.
	req, err = http.NewRequest("GET", "/exchange-code?code=good-code", nil)
	require.NoError(t, err)

	// Check that it responds with redirect to install app
	w = httptest.NewRecorder()
	controller.exchangeCode(w, req)
	assert.Equal(t, 302, w.Result().StatusCode)
	loc, err := w.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "/apps/etok/installations/new", loc.Path)

	// Check that credentials secret was created
	creds, err := client.Get(context.Background(), secretName, metav1.GetOptions{})
	assert.NoError(t, err)

	// Check contents of secret
	assert.Equal(t, "1", creds.StringData["id"])
	assert.Equal(t, "e340154128314309424b7c8e90325147d99fdafa", creds.StringData["webhook-secret"])
	assert.True(t, strings.HasPrefix(creds.StringData["key"], "-----BEGIN RSA PRIVATE KEY-----"))
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
