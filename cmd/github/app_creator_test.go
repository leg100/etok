package github

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/leg100/etok/cmd/github/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAppCreator(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, _ := fixtures.GithubServer(t)

	client := fake.NewClientBuilder().Build()

	go func() {
		creds := &credentials{
			namespace: "fake",
			timeout:   defaultTimeout,
			client:    client,
		}

		err := createApp(context.Background(), "test-app", "https://webhook.etok.dev", githubHostname, creds, createAppOptions{
			port:           12345,
			disableBrowser: true,
		})
		require.NoError(t, err)
	}()

	err := pollUrl(fmt.Sprintf("http://localhost:12345/healthz"), 10*time.Millisecond, 1*time.Second)
	require.NoError(t, err)

	resp, err := http.Get("http://localhost:12345/github-app/setup")
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	resp, err = http.Get("http://localhost:12345/exchange-code?code=good-code")
	content, err := ioutil.ReadAll(resp.Body)
	assert.Equal(t, "github app installation page", string(content))

	// Confirm exchange code redirected to GH
	loc, err := resp.Request.Response.Location()
	require.NoError(t, err)
	assert.Equal(t, "/apps/etok/installations/new", loc.Path)

	// Check that credentials secret was created
	secret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "fake", Name: secretName}}
	err = client.Get(context.Background(), runtimeclient.ObjectKeyFromObject(&secret), &secret)
	assert.NoError(t, err)

	// Check contents of secret
	assert.Equal(t, "1", secret.StringData["id"])
	assert.Equal(t, "e340154128314309424b7c8e90325147d99fdafa", secret.StringData["webhook-secret"])
	assert.True(t, strings.HasPrefix(secret.StringData["key"], "-----BEGIN RSA PRIVATE KEY-----"))
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
