package github

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/leg100/etok/cmd/github/fixtures"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// e2e test of the receipt of an event, to the trigger of etok run(s) and
// creating and updating github check run(s), through to completion
func TestE2E(t *testing.T) {
	disableSSLVerification(t)

	hdlr, checkRunObjs := webhookGithubTestServerRouter()
	githubHostname := webhookGithubTestServer(t, hdlr)

	ws1 := testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir"))

	// Create k8s clients
	cc := etokclient.NewFakeClientCreator(ws1)
	client, err := cc.Create("")
	require.NoError(t, err)

	// Setup mock repo
	path, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")

	app := newEtokRunApp(client, etokAppOptions{
		cloneDir: testutil.NewTempDir(t).Root(),
	})

	server := newWebhookServer(app)

	server.appID = 1
	server.keyPath = testutil.TempFile(t, "key", []byte(fixtures.GithubPrivateKey))
	server.githubHostname = githubHostname

	ctx, cancel := context.WithCancel(context.Background())
	errch := make(chan error)
	go func() {
		errch <- server.run(ctx)
	}()

	// Wait for dynamic port to be assigned
	for {
		if server.port != 0 {
			break
		}
	}

	req := fixtures.GitHubNewCheckSuiteEvent(t, server.port, sha, path)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	// Check that etok run(s) are successfully created
	err = wait.Poll(time.Millisecond*10, time.Second, func() (bool, error) {
		runs, err := client.RunsClient("default").List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		if len(runs.Items) == 1 {
			return true, nil
		}
		return false, nil
	})
	require.NoError(t, err)

	// Check that github checkrun(s) are successfully created
	<-checkRunObjs

	// Gracefully shutdown webhook server
	cancel()
	require.NoError(t, <-errch)
}
