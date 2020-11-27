package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				require.Equal(t, "", ws.Status.Active)
				require.Equal(t, []string{}, ws.Status.Queue)
			},
		},
		{
			name:      "Single command",
			workspace: testobj.Workspace("", "workspace-1"),
			objs: []runtime.Object{
				testobj.Run("", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			},
			assertions: func(ws *v1alpha1.Workspace) {
				require.Equal(t, "plan-1", ws.Status.Active)
				require.Equal(t, []string{}, ws.Status.Queue)
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
				require.Equal(t, "plan-1", ws.Status.Active)
				require.Equal(t, []string{"plan-2"}, ws.Status.Queue)
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
				require.Equal(t, "plan-1", ws.Status.Active)
				require.Equal(t, []string{"plan-2"}, ws.Status.Queue)
			},
		},
		{
			name:      "Garbage collected approval annotation",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithPrivilegedCommands("plan"), testobj.WithApprovals("plan-1")),
			assertions: func(ws *v1alpha1.Workspace) {
				require.Equal(t, map[string]string(nil), ws.Annotations)
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
				require.Equal(t, "1Gi", size.String())
			},
		},
		{
			name:      "Custom storage class",
			workspace: testobj.Workspace("", "workspace-1", testobj.WithStorageClass("local-path")),
			assertions: func(pvc *corev1.PersistentVolumeClaim) {
				require.Equal(t, "local-path", *pvc.Spec.StorageClassName)
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
				require.Equal(t, map[string]string{
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
				require.Equal(t, map[string]string{
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

func TestReconcileWorkspacePod(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		objs       []runtime.Object
		assertions func(pod *corev1.Pod)
	}{
		{
			name:      "Default",
			workspace: testobj.Workspace("controller-test", "workspace-1", testobj.WithHandshake("10s")),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t,
					[]string{"--", "sh", "-c", "terraform init -backend-config=backend.ini; terraform workspace select controller-test-workspace-1 || terraform workspace new controller-test-workspace-1"},
					pod.Spec.InitContainers[0].Args)

				assert.Equal(t, []corev1.EnvVar{
					{
						Name:  "STOK_HANDSHAKE",
						Value: "true",
					},
					{
						Name:  "STOK_HANDSHAKE_TIMEOUT",
						Value: "10s",
					},
				}, pod.Spec.InitContainers[0].Env)
			},
		},
		{
			name: "With credentials",
			objs: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "stok",
					},
					StringData: map[string]string{
						"AWS_ACCESS_KEY_ID":                   "youraccesskeyid",
						"AWS_SECRET_ACCESS_KEY":               "yoursecretaccesskey",
						"google_application_credentials.json": "abc",
					},
				},
			},
			workspace: testobj.Workspace("", "workspace-1", testobj.WithSecret("stok")),
			assertions: func(pod *corev1.Pod) {
				got, ok := getEnvValueForName(&pod.Spec.InitContainers[0], "GOOGLE_APPLICATION_CREDENTIALS")
				if !ok {
					t.Fatal("Could not find env var with name GOOGLE_APPLICATION_CREDENTIALS")
				}
				assert.Equal(t, "/credentials/google-credentials.json", got)

				assert.Equal(t, "stok", pod.Spec.InitContainers[0].EnvFrom[0].SecretRef.Name)
			},
		},
		{
			name:      "Pod Paths",
			workspace: testobj.Workspace("", "workspace-1"),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, "/workspace", pod.Spec.InitContainers[0].WorkingDir)

				var backendtf, backendini, dotterraform bool
				for _, vm := range pod.Spec.InitContainers[0].VolumeMounts {
					if vm.Name == "backendconfig" && vm.MountPath == "/workspace/backend.tf" {
						backendtf = true
					}
					if vm.Name == "backendconfig" && vm.MountPath == "/workspace/backend.ini" {
						backendini = true
					}
					if vm.Name == "cache" && vm.MountPath == "/workspace/.terraform" {
						dotterraform = true
					}
				}
				if !backendtf {
					t.Error("failed to find correct volume mount for backend.tf")
				}
				if !backendini {
					t.Error("failed to find correct volume mount for backend.ini")
				}
				if !dotterraform {
					t.Error("failed to find correct volume mount for .terraform/")
				}
			},
		},
		{
			name: "With service account",
			objs: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name: "stok",
					},
				},
			},
			workspace: testobj.Workspace("", "workspace-1", testobj.WithServiceAccount("stok")),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, "stok", pod.Spec.ServiceAccountName)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, append(tt.objs, tt.workspace)...)

			r := NewWorkspaceReconciler(cl, "a.b.c.d:v1")

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.workspace.GetName(),
					Namespace: tt.workspace.GetNamespace(),
				},
			}
			_, err := r.Reconcile(req)
			require.NoError(t, err)

			pod := &corev1.Pod{}
			podkey := types.NamespacedName{
				Name:      "workspace-" + tt.workspace.GetName(),
				Namespace: tt.workspace.GetNamespace(),
			}
			err = r.Get(context.TODO(), podkey, pod)
			require.NoError(t, err)

			tt.assertions(pod)
		})
	}
}
