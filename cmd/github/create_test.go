package github

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/leg100/etok/pkg/vcs/fixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreate(t *testing.T) {
	disableSSLVerification(t)

	githubHostname, err := fixtures.GithubAppTestServer(t)
	require.NoError(t, err)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "plan",
			args: []string{"--namespace=fake", "--disable-browser", fmt.Sprintf("--hostname=%s", githubHostname)},
		},
	}

	// Run tests for each command
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			out := new(bytes.Buffer)
			f := cmdutil.NewFakeFactory(out)

			cmd, opts := createCmd(f)
			cmd.SetArgs(tt.args)

			execErr := make(chan error)
			go func() {
				execErr <- cmd.Execute()
			}()

			// wait for listener to be started
			attempts := 5
			for i := 0; i < attempts; i++ {
				if opts.flow.port != 0 {
					break
				}
				if i == attempts {
					t.Error("listener failed to start")
					return
				}
				time.Sleep(100 * time.Millisecond)
			}

			healthzUrl := fmt.Sprintf("http://localhost:%d/healthz", opts.flow.port)
			require.NoError(t, pollUrl(healthzUrl, 100*time.Millisecond, 2*time.Second))

			// make request to exchange-code
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/exchange-code?code=good-code", opts.flow.port))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			content, err := ioutil.ReadAll(resp.Body)
			assert.Equal(t, "github app installation page", string(content))

			// Check that credentials secret was created
			_, err = opts.SecretsClient("fake").Get(context.Background(), secretName, metav1.GetOptions{})
			assert.NoError(t, err)

			// Check command completed without error
			assert.NoError(t, <-execErr)
		})
	}
}
