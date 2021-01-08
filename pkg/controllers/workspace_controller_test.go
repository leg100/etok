package controllers

import (
	"context"
	"io/ioutil"
	"testing"

	"sigs.k8s.io/yaml"

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileWorkspaceStatus(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		objs       []runtime.Object
		assertions func(ws *v1alpha1.Workspace)
	}{
		{
			name:      "No runs",
			workspace: testobj.Workspace("", "workspace-1"),
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string(nil), ws.Status.Queue)
			},
		},
		{
			name:      "Single command",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.WorkspacePod("", "workspace-1"),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1"}, ws.Status.Queue)
			},
		},
		{
			name:      "Three commands, one of which is unrelated to this workspace",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-3", "apply", testobj.WithWorkspace("workspace-2")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1", "apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Existing queue",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("apply-1")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1", "apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Completed command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("apply-3", "apply-1", "apply-2")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-3", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "apply-2", "apply", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1", "apply-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Completed command replaced by incomplete command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("apply-3")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
				testobj.Run("", "apply-3", "apply", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1"}, ws.Status.Queue)
			},
		},
		{
			name:      "Unapproved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply")),
			objs: []runtime.Object{
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string(nil), ws.Status.Queue)
			},
		},
		{
			name:      "Approved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply"), testobj.WithQueue("apply-1"), testobj.WithApprovals("apply-1")),
			objs: []runtime.Object{
				testobj.Run("", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"apply-1"}, ws.Status.Queue)
			},
		},
		{
			name:      "Garbage collected approval annotation",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("apply"), testobj.WithApprovals("apply-1")),
			objs: []runtime.Object{
				testobj.WorkspacePod("", "workspace-1"),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, map[string]string(nil), ws.Annotations)
			},
		},
		{
			name:      "Initializing phase",
			workspace: testobj.Workspace("", "workspace-1"),
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseInitializing, ws.Status.Phase)
			},
		},
		{
			name:      "Ready phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodRunning))},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseReady, ws.Status.Phase)
			},
		},
		{
			name:      "Error phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodSucceeded))},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
		},
		{
			name:      "Error phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodFailed))},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseError, ws.Status.Phase)
			},
		},
		{
			name:      "Unknown phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1", testobj.WithPhase(corev1.PodUnknown))},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, v1alpha1.WorkspacePhaseUnknown, ws.Status.Phase)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := append(tt.objs, runtime.Object(tt.workspace))
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, objs...)

			r := &WorkspaceReconciler{
				Client: cl,
				Scheme: scheme.Scheme,
			}

			req := requestFromObject(tt.workspace)
			_, err := r.Reconcile(context.Background(), req)
			require.NoError(t, err)

			// Fetch fresh workspace for assertions
			ws := &v1alpha1.Workspace{}
			require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))

			tt.assertions(ws)
		})
	}
}

func TestReconcileWorkspacePVC(t *testing.T) {
	var localPathStorageClass string = "local-path"

	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(pvc *corev1.PersistentVolumeClaim)
	}{
		{
			name:      "Default size",
			workspace: testobj.Workspace("", "workspace-1"),
			assertions: func(pvc *corev1.PersistentVolumeClaim) {
				size := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				assert.Equal(t, "1Gi", size.String())
			},
		},
		{
			name:      "Custom storage class",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithStorageClass(&localPathStorageClass)),
			assertions: func(pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, "local-path", *pvc.Spec.StorageClassName)
			},
		},
		{
			name:      "Owned",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithStorageClass(&localPathStorageClass)),
			assertions: func(pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, "Workspace", pvc.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", pvc.OwnerReferences[0].Name)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, tt.workspace)

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

			req := requestFromObject(tt.workspace)
			_, err := r.Reconcile(context.Background(), req)
			require.NoError(t, err)

			var pvc corev1.PersistentVolumeClaim
			err = r.Get(context.TODO(), req.NamespacedName, &pvc)
			require.NoError(t, err)

			tt.assertions(&pvc)
		})
	}
}

func TestReconcileWorkspacePod(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(*corev1.Pod)
	}{
		{
			name:      "Owned",
			workspace: testobj.Workspace("", "workspace-1"),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, "Workspace", pod.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", pod.OwnerReferences[0].Name)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, tt.workspace)

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

			_, err := r.Reconcile(context.Background(), requestFromObject(tt.workspace))
			require.NoError(t, err)

			var pod corev1.Pod
			podkey := types.NamespacedName{
				Name:      "workspace-" + tt.workspace.Name,
				Namespace: tt.workspace.Namespace,
			}
			require.NoError(t, r.Get(context.TODO(), podkey, &pod))

			tt.assertions(&pod)
		})
	}
}

func TestReconcileWorkspaceVariables(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(*corev1.ConfigMap)
	}{
		{
			name:      "Owned and has content",
			workspace: testobj.Workspace("", "workspace-1"),
			assertions: func(vars *corev1.ConfigMap) {
				assert.Equal(t, "Workspace", vars.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", vars.OwnerReferences[0].Name)
				assert.NotEmpty(t, vars.Data[variablesPath])
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, tt.workspace)

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

			_, err := r.Reconcile(context.Background(), requestFromObject(tt.workspace))
			require.NoError(t, err)

			var variables corev1.ConfigMap
			key := types.NamespacedName{
				Name:      tt.workspace.Name + "-variables",
				Namespace: tt.workspace.Namespace,
			}
			require.NoError(t, r.Get(context.TODO(), key, &variables))

			tt.assertions(&variables)
		})
	}
}

func TestReconcileWorkspaceState(t *testing.T) {
	testutil.Run(t, "outputs", func(t *testutil.T) {
		// Unmarshal YAML testdata into go object
		var state corev1.Secret
		f, err := ioutil.ReadFile("testdata/tfstate.yaml")
		require.NoError(t, yaml.Unmarshal(f, &state))

		var workspace = testobj.Workspace("default", "foo")
		cl := fake.NewFakeClientWithScheme(scheme.Scheme, workspace, &state)

		r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

		req := requestFromObject(workspace)
		_, err = r.Reconcile(context.Background(), req)
		require.NoError(t, err)

		// Fetch fresh workspace for assertions
		ws := &v1alpha1.Workspace{}
		require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))

		// Assert that state outputs have been persisted to workspace status
		assert.Equal(t, []*v1alpha1.Output{
			{
				Key:   "random_string",
				Value: "f584-default-foo-foo",
			},
		}, ws.Status.Outputs)

		// Assert that conditions have been updated accordingly
		assert.True(t, meta.IsStatusConditionFalse(ws.Status.Conditions, "StateFailure"))

		// Fetch fresh state secret for assertions
		state = corev1.Secret{}
		require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Namespace: "default", Name: ws.StateSecretName()}, &state))

		// Assert that workspace has been made owner of secret
		assert.Equal(t, "Workspace", state.OwnerReferences[0].Kind)
		assert.Equal(t, "foo", state.OwnerReferences[0].Name)
	})

	testutil.Run(t, "invalid state", func(t *testutil.T) {
		// A generic secret resource that is missing the 'tfstate' key and thus
		// should trigger an error
		state := testobj.Secret("default", "tfstate-default-foo")

		var workspace = testobj.Workspace("default", "foo")
		cl := fake.NewFakeClientWithScheme(scheme.Scheme, workspace, state)

		r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

		req := requestFromObject(workspace)
		_, err := r.Reconcile(context.Background(), req)
		require.NoError(t, err)

		// Fetch fresh workspace for assertions
		ws := &v1alpha1.Workspace{}
		require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))

		assert.True(t, meta.IsStatusConditionTrue(ws.Status.Conditions, "StateFailure"))
	})

	testutil.Run(t, "backup", func(t *testutil.T) {
		// Unmarshal YAML testdata into go object
		var state corev1.Secret
		f, err := ioutil.ReadFile("testdata/tfstate.yaml")
		require.NoError(t, yaml.Unmarshal(f, &state))

		server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
			InitialObjects: []fakestorage.Object{
				// Seems like the only way to programmatically create an initial
				// bucket is to create an initial object...
				{
					BucketName: "backup-bucket",
					Name:       "some/object/file.txt",
					Content:    []byte("inside the file"),
				},
			},
			Host: "127.0.0.1",
			Port: 8081,
		})
		require.NoError(t, err)
		defer server.Stop()

		testutil.Run(t.T, "valid bucket", func(t *testutil.T) {
			var workspace = testobj.Workspace("default", "foo")
			workspace.Spec.BackupBucket = "backup-bucket"

			cl := fake.NewFakeClientWithScheme(scheme.Scheme, workspace, &state)

			sclient := server.Client()

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1", WithStorageClient(sclient))

			req := requestFromObject(workspace)
			_, err = r.Reconcile(context.Background(), req)
			require.NoError(t, err)

			// Check object exists in bucket
			obj := sclient.Bucket("backup-bucket").Object(workspace.BackupObjectName())
			_, err = obj.Attrs(context.Background())
			require.NoError(t, err)

			// Read object
			or, err := obj.NewReader(context.Background())
			require.NoError(t, err)
			objectBytes, err := ioutil.ReadAll(or)
			require.NoError(t, err)

			// Unmarshal object
			var got corev1.Secret
			require.NoError(t, yaml.Unmarshal(objectBytes, &got))

			// Check testdata state is same as backup got from gcs
			assert.Equal(t, state, got)

			ws := &v1alpha1.Workspace{}
			require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))

			assert.True(t, meta.IsStatusConditionFalse(ws.Status.Conditions, "BackupFailure"))
		})

		testutil.Run(t.T, "invalid bucket", func(t *testutil.T) {
			var workspace = testobj.Workspace("default", "foo")
			workspace.Spec.BackupBucket = "does-not-exist"

			cl := fake.NewFakeClientWithScheme(scheme.Scheme, workspace, &state)

			sclient := server.Client()

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1", WithStorageClient(sclient))

			req := requestFromObject(workspace)
			_, err = r.Reconcile(context.Background(), req)
			require.NoError(t, err)

			// Fetch fresh workspace for assertions
			ws := &v1alpha1.Workspace{}
			require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))

			backupComplete := meta.FindStatusCondition(ws.Status.Conditions, "BackupFailure")
			if assert.NotNil(t, backupComplete) {
				assert.Equal(t, metav1.ConditionTrue, backupComplete.Status)
				assert.Equal(t, backupBucketNotFoundReason, backupComplete.Reason)
			}
		})
	})
}

func requestFromObject(obj client.Object) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
	}
}
