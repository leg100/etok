package testutil

import (
	"crypto/tls"
	"net/http"
	"testing"
)

// DisableSSLVerification disables ssl verification for the global http client
// for the duration of the test t
func DisableSSLVerification(t *testing.T) {
	orig := http.DefaultTransport.(*http.Transport).TLSClientConfig
	// nolint: gosec
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	t.Cleanup(func() {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = orig
	})
}
