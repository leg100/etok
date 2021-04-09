package github

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/leg100/etok/cmd/github/fixtures"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploy(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, _ := fixtures.GithubServer(t)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "plan",
			args: []string{"--namespace=fake", "--manifest-port=12345", "--wait=false", "--manifest-disable-browser", fmt.Sprintf("--hostname=%s", githubHostname), "--url=events.etok.dev"},
		},
	}

	// Run tests for each command
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			f := &cmdutil.Factory{
				IOStreams:            cmdutil.IOStreams{Out: out},
				RuntimeClientCreator: client.NewFakeRuntimeClientCreator(),
			}

			cmd, opts := deployCmd(f)
			cmd.SetArgs(tt.args)

			execErr := make(chan error)
			go func() {
				execErr <- cmd.Execute()
			}()

			// Wait for dynamic port to be assigned
			for {
				if opts.appCreatorOptions.port != 0 {
					break
				}
			}

			// Skip request to new app endpoint

			// Make request to exchange-code
			resp, err := http.Get("http://localhost:12345/exchange-code?code=good-code")
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			content, err := ioutil.ReadAll(resp.Body)
			assert.Equal(t, "github app installation page", string(content))

			// Check that credentials secret was created
			secret := corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "fake", Name: secretName}}
			err = opts.RuntimeClient.Get(context.Background(), runtimeclient.ObjectKeyFromObject(&secret), &secret)
			assert.NoError(t, err)

			// Check command completed without error
			assert.NoError(t, <-execErr)
		})
	}
}
