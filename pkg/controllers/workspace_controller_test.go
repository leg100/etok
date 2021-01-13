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
				assert.Equal(t, []string{"apply-1"}, ws.Status.Queue)
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
				assert.Equal(t, []string{"apply-1", "apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Queue with existing queue",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("apply-1")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1", "apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Approvals: Garbage collected approval annotation",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply"), testobj.WithApprovals("apply-1")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
			},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Nil(t, ws.Annotations)
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
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodRunning))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseInitializing, ws.Status.Phase)
			},
		},
		{
			name:      "Pod succeeded",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodSucceeded))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
		},
		{
			name:      "Pod failed",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodFailed))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
		},
		{
			name:      "Unknown phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodUnknown))},
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseUnknown, ws.Status.Phase)
			},
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
				testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodFailed)),
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
				assert.Equal(t, 4, ws.Status.BackupSerial)
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
				assert.Equal(t, 4, ws.Status.BackupSerial)
			}},
		{
			name:      "Non-existent backup bucket",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithBackupBucket("does-not-exist")),
			objs: []runtime.Object{
				testobj.Secret("dev", "tfstate-default-workspace-1", testobj.WithCompressedDataFromFile("tfstate", "testdata/tfstate.json")),
			},
			wantErr: true,
			workspaceAssertions: func(t *testutil.T, ws *v1alpha1.Workspace) {
				restoreFailure := meta.FindStatusCondition(ws.Status.Conditions, restoreFailureCondition)
				assert.Equal(t, metav1.ConditionTrue, restoreFailure.Status)
				assert.Equal(t, bucketNotFoundReason, restoreFailure.Reason)
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
			t.CheckError(tt.wantErr, err)

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

func TestWorkspacePhase(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		wantPhase  v1alpha1.WorkspacePhase
	}{
		{
			name:      "ready",
			wantPhase: v1alpha1.WorkspacePhaseReady,
		},
		{
			name: "initializing",
			conditions: []metav1.Condition{
				{
					Type:   podFailureCondition,
					Status: metav1.ConditionFalse,
					Reason: pendingReason,
				},
			},
			wantPhase: v1alpha1.WorkspacePhaseInitializing,
		},
		{
			name: "error",
			conditions: []metav1.Condition{
				{
					Type:   podFailureCondition,
					Status: metav1.ConditionTrue,
				},
			},
			wantPhase: v1alpha1.WorkspacePhaseError,
		},
		{
			name: "unknown",
			conditions: []metav1.Condition{
				{
					Type:   podFailureCondition,
					Status: metav1.ConditionUnknown,
				},
			},
			wantPhase: v1alpha1.WorkspacePhaseUnknown,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			ws := v1alpha1.Workspace{}
			ws.Status.Conditions = tt.conditions

			ws, _ = managePhase(context.Background(), ws)
			assert.Equal(t, tt.wantPhase, ws.Status.Phase)
		})
	}
}
