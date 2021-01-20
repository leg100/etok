package controllers

import (
	"context"
	"testing"

	"cloud.google.com/go/storage"

	"github.com/fsouza/fake-gcs-server/fakestorage"
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileWorkspace(t *testing.T) {
	var localPathStorageClass string = "local-path"

	tests := []struct {
		name                string
		workspace           *v1alpha1.Workspace
		objs                []runtime.Object
		bucketObjs          []fakestorage.Object
		workspaceAssertions func(*testutil.T, *v1alpha1.Workspace)
		podAssertions       func(*testutil.T, *corev1.Pod)
		pvcAssertions       func(*testutil.T, *corev1.PersistentVolumeClaim)
		configMapAssertions func(*testutil.T, *corev1.ConfigMap)
		stateAssertions     func(*testutil.T, *corev1.Secret)
		storageAssertions   func(*testutil.T, *storage.Client)
		wantErr             bool
	}{
		{
			name:      "Queue no runs",
			workspace: testobj.Workspace("", "workspace-1"),
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, []string(nil), ws.Status.Queue)
			},
		},
		{
			name:      "Queue single run",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.WorkspacePod("", "workspace-1"),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, "apply-1", ws.Status.Active)
			},
		},
		{
			name:      "Queue two runs",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-3", "apply", testobj.WithWorkspace("workspace-2")),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, "apply-1", ws.Status.Active)
				assert.Equal(t, []string{"apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Queue with existing queue",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithCombinedQueue("apply-1")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, "apply-1", ws.Status.Active)
				assert.Equal(t, []string{"apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Pruned approval annotation for completed run",
			workspace: testobj.Workspace("approvals", "workspace-1", testobj.WithPrivilegedCommands("apply"), testobj.WithApprovals("apply-1", "apply-2"), testobj.WithAnnotations("k1", "v1")),
			objs: []runtime.Object{
				testobj.WorkspacePod("approvals", "workspace-1"),
				testobj.Run("approvals", "apply-1", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				testobj.Run("approvals", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				want := map[string]string{"approvals.etok.dev/apply-2": "approved", "k1": "v1"}
				assert.Equal(t, want, ws.Annotations)
			},
		},
		{
			name:      "Initializing phase",
			workspace: testobj.Workspace("", "workspace-1"),
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseInitializing, ws.Status.Phase)
			},
		},
		{
			name:      "Ready phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodRunning)),
				testobj.PVC("", "workspace-1", testobj.WithPVCPhase(corev1.ClaimBound)),
				testobj.ConfigMap("", v1alpha1.WorkspaceVariablesConfigMapName("workspace-1")),
				&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "workspace-1"}},
				&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "workspace-1"}},
				testobj.Secret("", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseReady, ws.Status.Phase)
				assert.True(t, meta.IsStatusConditionTrue(ws.Status.Conditions, v1alpha1.WorkspaceReadyCondition))
			},
		},
		{
			name:      "Deleting phase",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithDeleteTimestamp()),
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseDeleting, ws.Status.Phase)
				if assert.True(t, meta.IsStatusConditionFalse(ws.Status.Conditions, v1alpha1.WorkspaceReadyCondition)) {
					ready := meta.FindStatusCondition(ws.Status.Conditions, v1alpha1.WorkspaceReadyCondition)
					assert.Equal(t, v1alpha1.DeletionReason, ready.Reason)
				}
			},
		},
		{
			name:      "Pod succeeded",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodSucceeded))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
			wantErr: true,
		},
		{
			name:      "Pod failed",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodFailed))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
			wantErr: true,
		},
		{
			name:      "Unknown phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodUnknown))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseUnknown, ws.Status.Phase)
			},
			wantErr: true,
		},
		{
			name:      "Cache: Default size",
			workspace: testobj.Workspace("", "workspace-1"),
			pvcAssertions: func(t *testutil.T, pvc *corev1.PersistentVolumeClaim) {
				size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				assert.Equal(t, "1Gi", size.String())
			},
		},
		{
			name:      "Cache: Custom storage class",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithStorageClass(&localPathStorageClass)),
			pvcAssertions: func(t *testutil.T, pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, "local-path", *pvc.Spec.StorageClassName)
			},
		},
		{
			name:      "Ownership of dependents",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithStorageClass(&localPathStorageClass)),
			objs: []runtime.Object{
				testobj.Secret("", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			configMapAssertions: func(t *testutil.T, vars *corev1.ConfigMap) {
				assert.Equal(t, "Workspace", vars.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", vars.OwnerReferences[0].Name)
			},
			podAssertions: func(t *testutil.T, pod *corev1.Pod) {
				assert.Equal(t, "Workspace", pod.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", pod.OwnerReferences[0].Name)
			},
			pvcAssertions: func(t *testutil.T, pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, "Workspace", pvc.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", pvc.OwnerReferences[0].Name)
			},
			stateAssertions: func(t *testutil.T, state *corev1.Secret) {
				assert.Equal(t, "Workspace", state.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", state.OwnerReferences[0].Name)
			},
		},
		{
			name:      "Variables: has data",
			workspace: testobj.Workspace("", "workspace-1"),
			configMapAssertions: func(t *testutil.T, vars *corev1.ConfigMap) {
				assert.NotEmpty(t, vars.Data[variablesPath])
			},
		},
		{
			name:      "Outputs",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodRunning)),
				testobj.Secret("", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, []*v1alpha1.Output{
					{
						Key:   "random_string",
						Value: "f584-default-foo-foo",
					},
				}, ws.Status.Outputs)
			},
		},
		{
			name:      "Backup",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithBackupBucket("backup-bucket")),
			objs: []runtime.Object{
				testobj.Secret("default", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			bucketObjs: []fakestorage.Object{
				{
					BucketName: "backup-bucket",
				},
			},
			storageAssertions: func(t *testutil.T, client *storage.Client) {
				// Check object exists in bucket
				obj := client.Bucket("backup-bucket").Object("default/workspace-1.yaml")
				_, err := obj.Attrs(context.Background())
				require.NoError(t, err)
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, 4, *ws.Status.BackupSerial)
			},
		},
		{
			name:      "Restore",
			workspace: testobj.Workspace("default", "workspace-1", testobj.WithBackupBucket("backup-bucket")),
			objs: []runtime.Object{
				testobj.Secret("default", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			bucketObjs: []fakestorage.Object{
				{
					BucketName: "backup-bucket",
					Name:       "default/workspace-1.yaml",
					Content:    readFile("testdata/tfstate.yaml"),
				},
			},
			storageAssertions: func(t *testutil.T, client *storage.Client) {
				// Check object exists in bucket
				obj := client.Bucket("backup-bucket").Object("default/workspace-1.yaml")
				_, err := obj.Attrs(context.Background())
				require.NoError(t, err)
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, 4, *ws.Status.BackupSerial)
			}},
		{
			name:      "Non-existent backup bucket",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithBackupBucket("does-not-exist")),
			objs: []runtime.Object{
				testobj.Secret("dev", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			wantErr: true,
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			objs := append(tt.objs, runtime.Object(tt.workspace))
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, objs...)

			// Setup up new fake GCS server for each test
			server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
				InitialObjects: tt.bucketObjs,
				Host:           "127.0.0.1",
				Port:           8081,
			})
			require.NoError(t, err)
			defer server.Stop()

			// Reconcile
			r := NewWorkspaceReconciler(cl, "", WithStorageClient(server.Client()))
			req := requestFromObject(tt.workspace)
			_, err = r.Reconcile(context.Background(), req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Fetch fresh workspace for assertions
			if tt.workspaceAssertions != nil {
				ws := &v1alpha1.Workspace{}
				require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))
				tt.workspaceAssertions(t, ws)
			}

			// Fetch fresh state secret for assertions
			if tt.stateAssertions != nil {
				state := corev1.Secret{}
				require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Namespace: tt.workspace.Namespace, Name: tt.workspace.StateSecretName()}, &state))
				tt.stateAssertions(t, &state)
			}

			if tt.configMapAssertions != nil {
				vars := corev1.ConfigMap{}
				require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Namespace: tt.workspace.Namespace, Name: tt.workspace.VariablesConfigMapName()}, &vars))
				tt.configMapAssertions(t, &vars)
			}

			if tt.podAssertions != nil {
				runner := corev1.Pod{}
				require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Namespace: tt.workspace.Namespace, Name: tt.workspace.PodName()}, &runner))
				tt.podAssertions(t, &runner)
			}

			if tt.pvcAssertions != nil {
				cache := corev1.PersistentVolumeClaim{}
				require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Namespace: tt.workspace.Namespace, Name: tt.workspace.PVCName()}, &cache))
				tt.pvcAssertions(t, &cache)
			}

			if tt.storageAssertions != nil {
				tt.storageAssertions(t, r.StorageClient)
			}
		})
	}
}
