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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAppCreator(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, _ := fixtures.GithubServer(t)

	client := fake.NewClientBuilder().Build()

	completed := make(chan error)
	go func() {
		creds := &credentials{
			namespace: "fake",
			timeout:   defaultTimeout,
			client:    client,
		}

		completed <- createApp(context.Background(), "test-app", "https://webhook.etok.dev", githubHostname, creds, createAppOptions{
			port:           12345,
			disableBrowser: true,
		})
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

	// Mimic github redirecting user after successful installation
	resp, err = http.Get("http://localhost:12345/github-app/installed?installation_id=16338139")
	content, err = ioutil.ReadAll(resp.Body)
	assert.Contains(t, string(content), "Github app installed successfully! You may now close this window.")

	// App creator should now automatically shut itself down
	require.NoError(t, <-completed)
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

// pollUrl polls a url every interval until timeout. If an HTTP 200 is received
// it exits without error.
func pollUrl(url string, interval, timeout time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		resp, err := http.Get(url)
		if err != nil {
			klog.V(2).Infof("polling %s: %s", url, err.Error())
			return false, nil
		}
		if resp.StatusCode == 200 {
			return true, nil
		}
		return false, nil
	})
}
