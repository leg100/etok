package github

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFlowServer(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, err := fixtures.GithubAppTestServer(t)
	require.NoError(t, err)

	opts := &flowServerOptions{
		githubHostname: githubHostname,
		name:           "etok-app",
		port:           12345,
		webhook:        "https://webhook.etok.dev",
		disableBrowser: true,
		creds: &credentials{
			name:      secretName,
			namespace: "fake",
			timeout:   defaultTimeout,
			client:    fake.NewSimpleClientset(),
		},
	}

	flow, err := newFlowServer(opts)
	require.NoError(t, err)

	errch := make(chan error)
	go func() {
		errch <- flow.run(context.Background())
	}()

	// Wait for flow server to startup
	<-flow.started

	w := httptest.NewRecorder()
	flow.newApp(w, nil)
	assert.Equal(t, 200, w.Result().StatusCode)

	// Check that it responds with redirect to install app
	w = httptest.NewRecorder()
	flow.exchangeCode(w, &http.Request{URL: &url.URL{RawQuery: "code=good-code"}})
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
