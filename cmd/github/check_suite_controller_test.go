package github

import (
	"context"
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/builders"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCheckSuiteController(t *testing.T) {
	tests := []struct {
		name               string
		suite              *v1alpha1.CheckSuite
		workspaces         []*v1alpha1.Workspace
		suiteAssertions    func(*testutil.T, *v1alpha1.CheckSuite)
		checkRunAssertions func(*testutil.T, *v1alpha1.CheckRunList)
	}{
		{
			name:  "Defaults",
			suite: builders.CheckSuite("12345").Build(),
			workspaces: []*v1alpha1.Workspace{
				testobj.Workspace("dev", "networks"),
				testobj.Workspace("prod", "networks"),
			},
			suiteAssertions: func(t *testutil.T, suite *v1alpha1.CheckSuite) {
				assert.NotEmpty(t, suite.Status.RepoPath)
			},
			checkRunAssertions: func(t *testutil.T, checkRuns *v1alpha1.CheckRunList) {
				assert.Equal(t, 2, len(checkRuns.Items))
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create a local mock of the upstream gh repo
			repo, sha := initializeRepo(t, "fixtures/repo")

			// Update CheckSuite resource with mock repo
			tt.suite.Spec.CloneURL = "file://" + repo
			tt.suite.Spec.SHA = sha
			tt.suite.Spec.Branch = "changes"
			tt.suite.Spec.Owner = "bob"
			tt.suite.Spec.Repo = "myrepo"
			tt.suite.Spec.InstallID = 12345

			bldr := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.suite)
			for _, ws := range tt.workspaces {
				ws.Spec.VCS.Repository = "file://" + repo
				bldr = bldr.WithObjects(ws)
			}

			cloneDir := t.NewTempDir().Root()
			reconciler := newCheckSuiteReconciler(bldr.Build(), &fakeTokenProvider{}, cloneDir)

			req := requestFromObject(tt.suite)
			_, err := reconciler.Reconcile(context.Background(), req)
			require.NoError(t, err)

			if tt.suiteAssertions != nil {
				require.NoError(t, reconciler.Get(context.Background(), req.NamespacedName, tt.suite))
				tt.suiteAssertions(t, tt.suite)
			}

			if tt.checkRunAssertions != nil {
				checkRuns := &v1alpha1.CheckRunList{}
				require.NoError(t, reconciler.List(context.Background(), checkRuns))
				tt.checkRunAssertions(t, checkRuns)
			}
		})
	}
}
