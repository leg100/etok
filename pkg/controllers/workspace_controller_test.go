package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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
				assert.Equal(t, []string{}, ws.Status.Queue)
			},
		},
		{
			name:      "Single command",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"plan-1"}, ws.Status.Queue)
			},
		},
		{
			name:      "Three commands, one of which is unrelated to this workspace",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "plan-2", "plan", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "plan-3", "plan", testobj.WithWorkspace("workspace-2")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"plan-1", "plan-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Existing queue",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("plan-1")),
			objs: []runtime.Object{
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "plan-2", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"plan-1", "plan-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Completed command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("plan-3", "plan-1", "plan-2")),
			objs: []runtime.Object{
				testobj.Run("", "plan-3", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
				testobj.Run("", "plan-2", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"plan-1", "plan-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Completed command replaced by incomplete command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithQueue("plan-3")),
			objs: []runtime.Object{
				testobj.Run("", "plan-3", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseCompleted)),
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"plan-1"}, ws.Status.Queue)
			},
		},
		{
			name:      "Unapproved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("plan")),
			objs: []runtime.Object{
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{}, ws.Status.Queue)
			},
		},
		{
			name:      "Approved privileged command",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("plan"), testobj.WithQueue("plan-1"), testobj.WithApprovals("plan-1")),
			objs: []runtime.Object{
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, []string{"plan-1"}, ws.Status.Queue)
			},
		},
		{
			name:      "Garbage collected approval annotation",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("plan"), testobj.WithApprovals("plan-1")),
			assertions: func(ws *v1alpha1.Workspace) {
				assert.Equal(t, map[string]string(nil), ws.Annotations)
			},
		},
		{
			name:      "Initializing phase",
			workspace: testobj.Workspace("", "workspace-1"),
			objs:      []runtime.Object{testobj.WorkspacePod("", "workspace-1")},
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
				Log:    ctrl.Log.WithName("controllers").WithName("Workspace"),
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.workspace.GetName(),
					Namespace: tt.workspace.GetNamespace(),
				},
			}
			_, err := r.Reconcile(req)
			require.NoError(t, err)

			// Fetch fresh workspace for assertions
			ws := &v1alpha1.Workspace{}
			require.NoError(t, r.Get(context.TODO(), req.NamespacedName, ws))

			tt.assertions(ws)
		})
	}
}

func TestReconcileWorkspacePVC(t *testing.T) {
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
			workspace: testobj.Workspace("", "workspace-1", testobj.WithStorageClass("local-path")),
			assertions: func(pvc *corev1.PersistentVolumeClaim) {
				assert.Equal(t, "local-path", *pvc.Spec.StorageClassName)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, tt.workspace)

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.workspace.GetName(),
					Namespace: tt.workspace.GetNamespace(),
				},
			}
			_, err := r.Reconcile(req)
			require.NoError(t, err)

			pvc := &corev1.PersistentVolumeClaim{}
			err = r.Get(context.TODO(), req.NamespacedName, pvc)
			require.NoError(t, err)

			tt.assertions(pvc)
		})
	}
}

func TestReconcileWorkspaceConfigMap(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(configmap *corev1.ConfigMap)
	}{
		{
			name:      "Default",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithBackendType("local")),
			assertions: func(configmap *corev1.ConfigMap) {
				assert.Equal(t, map[string]string{
					"backend.tf":  "terraform {\n  backend \"local\" {}\n}\n",
					"backend.ini": "",
				}, configmap.Data)
			},
		},
		{
			name: "GCS backend",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithBackendType("gcs"), testobj.WithBackendConfig(
				map[string]string{
					"bucket": "workspace-1-state",
					"prefix": "dev",
				},
			)),
			assertions: func(configmap *corev1.ConfigMap) {
				assert.Equal(t, map[string]string{
					"backend.tf": "terraform {\n  backend \"gcs\" {}\n}\n",
					"backend.ini": "bucket	= \"workspace-1-state\"\nprefix	= \"dev\"\n",
				}, configmap.Data)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, tt.workspace)

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.workspace.GetName(),
					Namespace: tt.workspace.GetNamespace(),
				},
			}
			_, err := r.Reconcile(req)
			require.NoError(t, err)

			configmap := &corev1.ConfigMap{}
			configmapkey := types.NamespacedName{
				Name:      "workspace-" + tt.workspace.GetName(),
				Namespace: tt.workspace.GetNamespace(),
			}
			err = r.Get(context.TODO(), configmapkey, configmap)
			require.NoError(t, err)

			tt.assertions(configmap)
		})
	}
}

func envsToMap(envs []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(envs))
	for _, ev := range envs {
		m[ev.Name] = ev.Value
	}
	return m
}
