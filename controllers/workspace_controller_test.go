package controllers

import (
	"context"
	"reflect"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var workspaceEmptyQueue2 = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName:         "stok",
		ServiceAccountName: "stok",
	},
}

var workspaceWithoutSecret = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		ServiceAccountName: "stok",
	},
}

var workspaceWithCacheSpec = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		Cache: v1alpha1.WorkspaceCacheSpec{
			Size:         "2Gi",
			StorageClass: "local-path",
		},
		SecretName:         "stok",
		ServiceAccountName: "stok",
	},
}

var workspaceWithQueue = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName:         "stok",
		ServiceAccountName: "stok",
	},
}

var secret2 = corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "stok",
		Namespace: "operator-test",
		Labels: map[string]string{
			"app": "stok",
		},
	},
}

var serviceAccount = corev1.ServiceAccount{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "stok",
		Namespace: "operator-test",
		Labels: map[string]string{
			"app": "stok",
		},
	},
}

func TestReconcileWorkspace(t *testing.T) {
	plan1 := v1alpha1.Plan{}
	plan1.SetName("plan-1")
	plan1.SetNamespace("operator-test")
	plan1.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": "workspace-1",
	})

	plan2 := v1alpha1.Plan{}
	plan2.SetName("plan-2")
	plan2.SetNamespace("operator-test")
	plan2.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": "workspace-1",
	})

	planWithNonExistantWorkspace := v1alpha1.Plan{}
	planWithNonExistantWorkspace.SetName("pod-with-non-existant-workspace")
	planWithNonExistantWorkspace.SetNamespace("operator-test")
	planWithNonExistantWorkspace.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": "workspace-does-not-exist",
	})

	planCompleted := v1alpha1.Plan{}
	planCompleted.SetName("plan-3")
	planCompleted.SetNamespace("operator-test")
	planCompleted.SetLabels(map[string]string{
		"app":       "stok",
		"workspace": "workspace-1",
	})
	planCompleted.Conditions.SetCondition(
		status.Condition{
			Type:   v1alpha1.ConditionCompleted,
			Status: corev1.ConditionTrue,
		},
	)

	tests := []struct {
		name                  string
		workspace             *v1alpha1.Workspace
		objs                  []runtime.Object
		status                v1alpha1.WorkspaceStatus
		wantQueue             []string
		wantRequeue           bool
		wantPVC               bool
		wantCacheSize         string
		wantCacheStorageClass string
		wantHealthyCondition  corev1.ConditionStatus
	}{
		{
			name:      "Missing secret",
			workspace: &workspaceEmptyQueue2,
			objs: []runtime.Object{
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionFalse,
		},
		{
			name:      "Missing service account",
			workspace: &workspaceEmptyQueue2,
			objs: []runtime.Object{
				runtime.Object(&secret2),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionFalse,
		},
		{
			name:      "No secret",
			workspace: &workspaceWithoutSecret,
			objs: []runtime.Object{
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
			wantPVC:              true,
		},
		{
			name:      "Workspace with cache spec",
			workspace: &workspaceWithCacheSpec,
			objs: []runtime.Object{
				runtime.Object(&secret2),
				runtime.Object(&serviceAccount),
			},
			wantQueue:             []string{},
			wantRequeue:           false,
			wantCacheSize:         "2Gi",
			wantCacheStorageClass: "local-path",
			wantHealthyCondition:  corev1.ConditionTrue,
			wantPVC:               true,
		},
		{
			name:      "No commands",
			workspace: &workspaceEmptyQueue2,
			objs: []runtime.Object{
				runtime.Object(&planWithNonExistantWorkspace),
				runtime.Object(&secret2),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
			wantPVC:              true,
		},
		{
			name:      "Single command",
			workspace: &workspaceEmptyQueue2,
			objs: []runtime.Object{
				runtime.Object(&plan1),
				runtime.Object(&secret2),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{"plan-1"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
			wantPVC:              true,
		},
		{
			name:      "Two commands",
			workspace: &workspaceEmptyQueue2,
			objs: []runtime.Object{
				runtime.Object(&plan1),
				runtime.Object(&plan2),
				runtime.Object(&secret2),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{"plan-1", "plan-2"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
			wantPVC:              true,
		},
		{
			name:      "Existing queue",
			workspace: &workspaceWithQueue,
			objs: []runtime.Object{
				runtime.Object(&plan1),
				runtime.Object(&plan2),
				runtime.Object(&secret2),
				runtime.Object(&serviceAccount),
			},
			status: v1alpha1.WorkspaceStatus{
				Queue: []string{
					"plan-1",
				},
			},
			wantQueue:            []string{"plan-1", "plan-2"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
			wantPVC:              true,
		},
		{
			name:      "Completed command",
			workspace: &workspaceEmptyQueue2,
			objs: []runtime.Object{
				runtime.Object(&planCompleted),
				runtime.Object(&plan1),
				runtime.Object(&plan2),
				runtime.Object(&secret2),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{"plan-1", "plan-2"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
			wantPVC:              true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.workspace.Status = tt.status
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
			res, err := r.Reconcile(req)
			if err != nil {
				t.Fatal(err)
			}

			if tt.wantRequeue && !res.Requeue {
				t.Error("expected reconcile to requeue")
			}

			err = r.Get(context.TODO(), req.NamespacedName, tt.workspace)
			if err != nil {
				t.Fatalf("get ws: (%v)", err)
			}

			gotHealthyCondition := tt.workspace.Status.Conditions.GetCondition(v1alpha1.ConditionHealthy)
			if tt.wantHealthyCondition != gotHealthyCondition.Status {
				t.Fatalf("want %s got %s", tt.wantHealthyCondition, gotHealthyCondition.Status)
			}

			queue := tt.workspace.Status.Queue
			if !reflect.DeepEqual(tt.wantQueue, queue) {
				t.Fatalf("workspace queue expected to be %#v, but got %#v", tt.wantQueue, queue)
			}

			if tt.wantPVC {
				pvc := &corev1.PersistentVolumeClaim{}
				err = r.Get(context.TODO(), req.NamespacedName, pvc)
				if err != nil {
					t.Errorf("get pvc: (%v)", err)
				}

				// If not set, want default of 1Gi
				if tt.wantCacheSize == "" {
					tt.wantCacheSize = "1Gi"
				}
				gotSize, _ := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				if tt.wantCacheSize != gotSize.String() {
					t.Errorf("want %s got %s", tt.wantCacheSize, gotSize.String())
				}

				// Don't test when StorageClass is unset
				if tt.wantCacheStorageClass != "" {
					gotStorageClass := pvc.Spec.StorageClassName
					if tt.wantCacheStorageClass != *gotStorageClass {
						t.Errorf("want %s got %s", tt.wantCacheStorageClass, *gotStorageClass)
					}
				}
			}
		})
	}
}
