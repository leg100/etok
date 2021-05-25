package github

import (
	"context"
	"testing"

	"github.com/google/go-github/v31/github"
	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHandleEvent(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		objs       []runtime.Object
		event      interface{}
		assertions func(*testutil.T, runtimeclient.Client)
	}{
		{
			name: "checksuite requested",
			event: &github.CheckSuiteEvent{
				Action: github.String("requested"),
				CheckSuite: &github.CheckSuite{
					HeadBranch: github.String("changes"),
					PullRequests: []*github.PullRequest{
						{},
					},
				},
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				suites := &v1alpha1.CheckSuiteList{}
				require.NoError(t, client.List(context.Background(), suites))
				require.Equal(t, 1, len(suites.Items))

				assert.NotEmpty(t, suites.Items[0].Spec.CloneURL)
			},
		},
		{
			name: "checksuite rerequested",
			event: &github.CheckSuiteEvent{
				Action: github.String("rerequested"),
				CheckSuite: &github.CheckSuite{
					ID:         github.Int64(123),
					HeadBranch: github.String("changes"),
					PullRequests: []*github.PullRequest{
						{},
					},
				},
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
			},
			objs: []runtime.Object{
				&v1alpha1.CheckSuite{ObjectMeta: metav1.ObjectMeta{Name: "123"}},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				suites := &v1alpha1.CheckSuiteList{}
				require.NoError(t, client.List(context.Background(), suites))
				require.Equal(t, 1, len(suites.Items))

				assert.Equal(t, 1, suites.Items[0].Spec.Rerequests)
			},
		},
		{
			name: "checkrun rerequested plan",
			event: &github.CheckRunEvent{
				Action: github.String("rerequested"),
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						ID:         github.Int64(123456),
						HeadBranch: github.String("changes"),
					},
					ExternalID: github.String("abc/def"),
				},
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
			},
			objs: []runtime.Object{
				&v1alpha1.CheckRun{ObjectMeta: metav1.ObjectMeta{Namespace: "abc", Name: "def"}},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				cr := &v1alpha1.CheckRun{}
				require.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "abc", Name: "def"}, cr))
				assert.Equal(t, &v1alpha1.CheckRunRerequestedEvent{}, cr.Status.Events[0].Rerequested)
			},
		},
		{
			name: "checkrun requested_action plan event",
			event: &github.CheckRunEvent{
				Action: github.String("requested_action"),
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						ID:         github.Int64(123456),
						HeadBranch: github.String("changes"),
					},
					ExternalID: github.String("abc/def"),
				},
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
				RequestedAction: &github.RequestedAction{
					Identifier: "plan",
				},
			},
			objs: []runtime.Object{
				&v1alpha1.CheckRun{ObjectMeta: metav1.ObjectMeta{Namespace: "abc", Name: "def"}},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				cr := &v1alpha1.CheckRun{}
				require.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "abc", Name: "def"}, cr))
				assert.Equal(t, &v1alpha1.CheckRunRequestedActionEvent{Action: "plan"}, cr.Status.Events[0].RequestedAction)
			},
		},
		{
			name: "checkrun requested_action apply event",
			event: &github.CheckRunEvent{
				Action: github.String("requested_action"),
				CheckRun: &github.CheckRun{
					CheckSuite: &github.CheckSuite{
						ID:         github.Int64(123456),
						HeadBranch: github.String("changes"),
					},
					ExternalID: github.String("abc/def"),
				},
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
				RequestedAction: &github.RequestedAction{
					Identifier: "apply",
				},
			},
			objs: []runtime.Object{
				&v1alpha1.CheckRun{ObjectMeta: metav1.ObjectMeta{Namespace: "abc", Name: "def"}},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				cr := &v1alpha1.CheckRun{}
				require.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "abc", Name: "def"}, cr))
				assert.Equal(t, &v1alpha1.CheckRunRequestedActionEvent{Action: "apply"}, cr.Status.Events[0].RequestedAction)
			},
		},
		{
			name: "checkrun created event",
			event: &github.CheckRunEvent{
				Action: github.String("created"),
				CheckRun: &github.CheckRun{
					ID: github.Int64(123456),
					CheckSuite: &github.CheckSuite{
						ID:         github.Int64(123456),
						HeadBranch: github.String("changes"),
					},
					ExternalID: github.String("abc/def"),
				},
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
			},
			objs: []runtime.Object{
				&v1alpha1.CheckRun{ObjectMeta: metav1.ObjectMeta{Namespace: "abc", Name: "def"}},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				cr := &v1alpha1.CheckRun{}
				require.NoError(t, client.Get(context.Background(), runtimeclient.ObjectKey{Namespace: "abc", Name: "def"}, cr))
				assert.Equal(t, &v1alpha1.CheckRunCreatedEvent{ID: 123456}, cr.Status.Events[0].Created)
			},
		},
		{
			name: "pull request open event",
			event: &github.PullRequestEvent{
				Action: github.String("opened"),
				Repo: &github.Repository{
					Name: github.String("myrepo"),
					Owner: &github.User{
						Login: github.String("bob"),
					},
				},
				PullRequest: &github.PullRequest{
					Head: &github.PullRequestBranch{
						Ref: github.String("changes"),
					},
				},
			},
			objs: []runtime.Object{
				&v1alpha1.CheckRun{ObjectMeta: metav1.ObjectMeta{Namespace: "abc", Name: "def"}},
			},
			assertions: func(t *testutil.T, client runtimeclient.Client) {
				suites := &v1alpha1.CheckSuiteList{}
				require.NoError(t, client.List(context.Background(), suites))
				require.Equal(t, 1, len(suites.Items))

				assert.NotEmpty(t, suites.Items[0].Spec.CloneURL)
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create a local mock of the upstream gh repo
			repo, sha := initializeRepo(t, "fixtures/repo")

			// Update event to make it look like it came from repo
			switch ev := tt.event.(type) {
			case *github.CheckSuiteEvent:
				ev.CheckSuite.HeadSHA = &sha
				ev.Repo.CloneURL = github.String("file://" + repo)
			case *github.CheckRunEvent:
				ev.CheckRun.CheckSuite.HeadSHA = &sha
				ev.Repo.CloneURL = github.String("file://" + repo)
			}

			// Connect up workspaces to mock repo
			for _, obj := range tt.objs {
				ws, ok := obj.(*v1alpha1.Workspace)
				if ok {
					ws.Spec.VCS.Repository = "file://" + repo
				}
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithRuntimeObjects(tt.objs...).
				Build()

			checksClient := &fakeChecksClient{
				cloneURL: "file://" + repo,
				sha:      sha,
			}

			require.NoError(t, newApp(client).handleEvent(tt.event, checksClient))

			tt.assertions(t, client)
		})
	}
}

type fakeChecksClient struct {
	cloneURL string
	sha      string
}

func (c *fakeChecksClient) ListCheckSuitesForRef(ctx context.Context, owner, repo, ref string, opts *github.ListCheckSuiteOptions) (*github.ListCheckSuiteResults, *github.Response, error) {
	results := &github.ListCheckSuiteResults{
		Total: github.Int(1),
		CheckSuites: []*github.CheckSuite{
			{
				ID: github.Int64(123),
				Repository: &github.Repository{
					CloneURL: &c.cloneURL,
					Name:     &repo,
					Owner: &github.User{
						Login: &owner,
					},
				},
				HeadBranch: &ref,
				HeadSHA:    &c.sha,
			},
		},
	}

	return results, nil, nil
}
