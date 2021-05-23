package github

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	githubclient "github.com/leg100/etok/cmd/github/client"
	"github.com/leg100/etok/pkg/builders"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestCheckRunController(t *testing.T) {
	tests := []struct {
		name             string
		cr               *v1alpha1.CheckRun
		objs             []runtime.Object
		assertions       func(*testutil.T, *checkRunUpdate)
		clientAssertions func(*testutil.T, client.Client)
	}{
		{
			name: "New check run",
			cr:   builders.CheckRun("dev/12345-networks").Build(),
			objs: []runtime.Object{
				testobj.Workspace("dev", "networks", testobj.WithWorkingDir("networks")),
			},
			assertions: func(t *testutil.T, u *checkRunUpdate) {
				assert.NotNil(t, u)
			},
			clientAssertions: func(t *testutil.T, c client.Client) {
				cr := builders.CheckRun("dev/12345-networks").Build()
				require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(cr), cr))
				assert.True(t, meta.IsStatusConditionTrue(cr.Status.Conditions, "CreateRequested"))
				assert.Equal(t, 1, len(cr.Status.Iterations))
				assert.False(t, cr.Status.Iterations[0].Completed)

				run := testobj.Run("dev", "12345-networks-0", "sh")
				require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(run), run))

				configMap := testobj.ConfigMap("dev", "12345-networks-0")
				require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(configMap), configMap))
			},
		},
		{
			name: "Ensure update is not sent if create has been requested but not yet created",
			cr:   builders.CheckRun("dev/12345-networks").CreateRequested().Build(),
			objs: []runtime.Object{
				testobj.Workspace("dev", "networks", testobj.WithWorkingDir("networks")),
			},
			assertions: func(t *testutil.T, u *checkRunUpdate) {
				assert.Nil(t, u)
			},
		},
		{
			name: "Ensure update is sent if create has been requested and it has been created",
			cr:   builders.CheckRun("dev/12345-networks").CreateRequested().ID(123).Build(),
			objs: []runtime.Object{
				testobj.Workspace("dev", "networks", testobj.WithWorkingDir("networks")),
			},
			assertions: func(t *testutil.T, u *checkRunUpdate) {
				assert.NotNil(t, u)
			},
		},
		{
			name: "Streamable",
			cr:   builders.CheckRun("dev/12345-networks").Build(),
			objs: []runtime.Object{
				testobj.Workspace("dev", "networks", testobj.WithWorkingDir("netwoks")),
				testobj.Run("dev", "12345-networks-0", "sh", testobj.WithCondition(v1alpha1.RunCompleteCondition, v1alpha1.PodRunningReason)),
			},
			assertions: func(t *testutil.T, u *checkRunUpdate) {
				assert.Equal(t, []byte("fake logs"), u.logs)
			},
		},
		{
			name: "Completed iteration",
			cr:   builders.CheckRun("dev/12345-networks").Build(),
			objs: []runtime.Object{
				testobj.Workspace("dev", "networks", testobj.WithWorkingDir("netwoks")),
				testobj.Run("dev", "12345-networks-0", "sh", testobj.WithCondition(v1alpha1.RunCompleteCondition, v1alpha1.PodSucceededReason)),
			},
			clientAssertions: func(t *testutil.T, c client.Client) {
				cr := builders.CheckRun("dev/12345-networks").Build()
				require.NoError(t, c.Get(context.Background(), client.ObjectKeyFromObject(cr), cr))
				assert.Equal(t, 1, len(cr.Status.Iterations))
				assert.True(t, cr.Status.Iterations[0].Completed)
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			// Create a checksuite pointing at a repo
			path := t.NewTempDir().Mkdir("clone123/networks").Touch("clone123/networks/main.tf").Root()
			suite := builders.CheckSuite("12345").RepoPath(filepath.Join(path, "clone123")).Build()

			// Build a fake client populated with objs
			client := fake.NewClientBuilder().
				WithRuntimeObjects(tt.cr).
				WithRuntimeObjects(tt.objs...).
				WithRuntimeObjects(suite).
				WithScheme(scheme.Scheme).
				Build()

			sender := &fakeSender{}
			reconciler := &checkRunReconciler{
				Client:   client,
				sender:   sender,
				streamer: &fakeStreamer{},
			}

			req := requestFromObject(tt.cr)
			_, err := reconciler.Reconcile(context.Background(), req)
			require.NoError(t, err)

			if tt.assertions != nil {
				tt.assertions(t, sender.u)
			}
			if tt.clientAssertions != nil {
				tt.clientAssertions(t, client)
			}
		})
	}
}

func requestFromObject(obj client.Object) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
	}
}

type fakeStreamer struct{}

func (s *fakeStreamer) Stream(ctx context.Context, key runtimeclient.ObjectKey) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBufferString("fake logs")), nil
}

type fakeSender struct {
	u *checkRunUpdate
}

func (s *fakeSender) Send(_ int64, _ string, inv githubclient.Invokable) error {
	s.u = inv.(*checkRunUpdate)

	return nil
}
