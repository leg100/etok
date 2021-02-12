package github

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
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

func TestFlowServer(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, err := fixtures.GithubAppTestServer(t)
	require.NoError(t, err)

	flow := &flowServer{
		githubHostname: githubHostname,
		name:           "etok-app",
		port:           12345,
		webhook:        "https://webhook.etok.dev",
		Render: render.New(
			render.Options{
				Asset:      static.Asset,
				AssetNames: static.AssetNames,
				Directory:  "static/templates",
			},
		),
		creds: &credentials{
			name:    secretName,
			timeout: defaultTimeout,
			client:  fake.NewSimpleClientset(),
		},
	}

	errch := make(chan error)
	go func() {
		errch <- flow.run(context.Background())
	}()

	req, err := http.NewRequest("GET", "/github-app/setup", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	flow.newApp(w, req)
	assert.Equal(t, 200, w.Result().StatusCode)

	// Make request for exchange code.
	req, err = http.NewRequest("GET", "/exchange-code?code=good-code", nil)
	require.NoError(t, err)

	// Check that it responds with redirect to install app
	w = httptest.NewRecorder()
	flow.exchangeCode(w, req)
	assert.Equal(t, 302, w.Result().StatusCode)
	loc, err := w.Result().Location()
	assert.NoError(t, err)
	assert.Equal(t, "/apps/etok/installations/new", loc.Path)

	// Check that credentials secret was created
	creds, err := flow.creds.client.CoreV1().Secrets("fake").Get(context.Background(), secretName, metav1.GetOptions{})
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
