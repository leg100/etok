package github

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/github/fixtures"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/logstreamer"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// e2e test of the receipt of an event, to the trigger of etok run(s) and
// creating and updating github check run(s), through to completion
func TestE2E(t *testing.T) {
	disableSSLVerification(t)

	// Start a mock github API, which sends any checkrun updates it receives via
	// the checkRunObjs channel
	githubHostname, checkRunObjs := fixtures.GithubServer(t)

	ws1 := testobj.Workspace("default", "default", testobj.WithRepository("bob/myrepo"), testobj.WithBranch("changes"), testobj.WithWorkingDir("subdir"))

	// Create k8s clients
	cc := etokclient.NewFakeClientCreator(ws1)
	client, err := cc.Create("")
	require.NoError(t, err)

	// Create our special log streamer
	logStreamer := make(getLogsFunc)

	app := newEtokRunApp(client, etokAppOptions{
		cloneDir:    testutil.NewTempDir(t).Root(),
		getLogsFunc: logStreamer.getLogs,
		// Simulate run as having successfully completed
		runStatus: v1alpha1.RunStatus{
			ExitCode: github.Int(0),
			Phase:    v1alpha1.RunPhaseCompleted,
			Conditions: []metav1.Condition{
				{
					Type:   v1alpha1.RunCompleteCondition,
					Status: metav1.ConditionFalse,
					Reason: v1alpha1.PodRunningReason,
				},
			},
		},
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

	// Setup mock repo
	path, sha := initializeRepo(&testutil.T{T: t}, "./fixtures/repo")

	// Send event, kicking off e2e process
	req := fixtures.GitHubNewCheckSuiteEvent(t, server.port, sha, path)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)

	// Ensure github checkruns are successfully created
	cr := <-checkRunObjs
	createCheckRunOpts, ok := cr.(github.CreateCheckRunOptions)
	require.True(t, ok)
	assert.Equal(t, "in_progress", *createCheckRunOpts.Status)

	// Now send message to log streamer, giving the go-ahead for it to print
	// logs
	logStreamer <- struct{}{}

	// Check run should now be updated. There may be more than one update, but
	// the last one should have a completed status.
outer:
	for cr := range checkRunObjs {
		switch cr := cr.(type) {
		case github.UpdateCheckRunOptions:
			switch status := *cr.Status; status {
			case "in_progress":
				t.Log("received check run with status in_progress")
			case "completed":
				t.Log("received check run with status completed")
				assert.Equal(t, "default/default | plan (no changes)", cr.Name)
				assert.Equal(t, github.String("success"), cr.Conclusion)
				break outer
			default:
				t.Logf("received check run with unexpected status: %s", status)
				t.FailNow()
			}
		}
	}

	// Expect one etok run to have been created
	runs, err := client.RunsClient("default").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, len(runs.Items))

	// Gracefully shutdown webhook server
	cancel()
	require.NoError(t, <-errch)
}

// A special log streamer for testing purposes. We want to control the timing of
// a run and by having the log streaming wait for a message on a channel we can
// delay the run from printing out logs and finishing. In this way, we can
// ensure at least two check run updates: at least one before the message is
// sent, and at least one after the message is sent. And we want two check run
// updates to demonstrate that the first one *creates* the check run, and that
// the second is an *update* to the first.
type getLogsFunc chan struct{}

func (f getLogsFunc) getLogs(ctx context.Context, opts logstreamer.Options) (io.ReadCloser, error) {
	// Wait for the message to proceed
	<-f

	// Mimic terraform plan showing no changes
	return ioutil.NopCloser(bytes.NewBufferString("No changes. Infrastructure is up-to-date.\n")), nil
}
