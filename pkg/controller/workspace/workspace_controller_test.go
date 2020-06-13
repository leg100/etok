package workspace

import (
	"context"
	"reflect"
	"testing"

	"github.com/leg100/stok/pkg/apis"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var workspaceEmptyQueue = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName:         "stok",
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

var secret = corev1.Secret{
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

var pod1 = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-1",
		Namespace: "operator-test",
		Labels: map[string]string{
			"app":       "stok",
			"workspace": "workspace-1",
		},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
	},
}

var pod2 = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-2",
		Namespace: "operator-test",
		Labels: map[string]string{
			"app":       "stok",
			"workspace": "workspace-1",
		},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
	},
}

var completedPod = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-3",
		Namespace: "operator-test",
		Labels: map[string]string{
			"app":       "stok",
			"workspace": "workspace-1",
		},
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodSucceeded,
	},
}

var podWithNonExistantWorkspace = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "pod-with-nonexistant-workspace",
		Namespace: "operator-test",
		Labels: map[string]string{
			"app":       "stok",
			"workspace": "workspace-does-not-exist",
		},
	},
}

func TestReconcileWorkspace(t *testing.T) {
	tests := []struct {
		name                  string
		workspace             *v1alpha1.Workspace
		objs                  []runtime.Object
		status                v1alpha1.WorkspaceStatus
		wantQueue             []string
		wantRequeue           bool
		wantCacheSize         string
		wantCacheStorageClass string
		wantHealthyCondition  corev1.ConditionStatus
	}{
		{
			name:      "Missing secret",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionFalse,
		},
		{
			name:      "Missing service account",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&secret),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionFalse,
		},
		{
			name:      "Workspace with cache spec",
			workspace: &workspaceWithCacheSpec,
			objs: []runtime.Object{
				runtime.Object(&secret),
				runtime.Object(&serviceAccount),
			},
			wantQueue:             []string{},
			wantRequeue:           false,
			wantCacheSize:         "2Gi",
			wantCacheStorageClass: "local-path",
			wantHealthyCondition:  corev1.ConditionTrue,
		},
		{
			name:      "No commands",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&podWithNonExistantWorkspace),
				runtime.Object(&secret),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
		},
		{
			name:      "Single command",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&pod1),
				runtime.Object(&secret),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{"pod-1"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
		},
		{
			name:      "Two commands",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&pod1),
				runtime.Object(&pod2),
				runtime.Object(&secret),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{"pod-1", "pod-2"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
		},
		{
			name:      "Existing queue",
			workspace: &workspaceWithQueue,
			objs: []runtime.Object{
				runtime.Object(&pod1),
				runtime.Object(&pod2),
				runtime.Object(&secret),
				runtime.Object(&serviceAccount),
			},
			status: v1alpha1.WorkspaceStatus{
				Queue: []string{
					"pod-1",
				},
			},
			wantQueue:            []string{"pod-1", "pod-2"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
		},
		{
			name:      "Completed command",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&completedPod),
				runtime.Object(&pod1),
				runtime.Object(&pod2),
				runtime.Object(&secret),
				runtime.Object(&serviceAccount),
			},
			wantQueue:            []string{"pod-1", "pod-2"},
			wantRequeue:          false,
			wantHealthyCondition: corev1.ConditionTrue,
		},
	}
	s := scheme.Scheme
	apis.AddToScheme(s)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.workspace.Status = tt.status
			objs := append(tt.objs, runtime.Object(tt.workspace))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			r := &ReconcileWorkspace{client: cl, scheme: s}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.workspace.GetName(),
					Namespace: tt.workspace.GetNamespace(),
				},
			}
			res, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("reconcile: (%v)", err)
			}

			if tt.wantRequeue && !res.Requeue {
				t.Error("expected reconcile to requeue")
			}

			pvc := &corev1.PersistentVolumeClaim{}
			err = r.client.Get(context.TODO(), req.NamespacedName, pvc)
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

			err = r.client.Get(context.TODO(), req.NamespacedName, tt.workspace)
			if err != nil {
				t.Fatalf("get ws: (%v)", err)
			}

			gotHealthyCondition := tt.workspace.Status.Conditions.GetCondition(status.ConditionType("Healthy"))
			if tt.wantHealthyCondition != gotHealthyCondition.Status {
				t.Fatalf("want %s got %s", tt.wantHealthyCondition, gotHealthyCondition.Status)
			}

			queue := tt.workspace.Status.Queue
			if !reflect.DeepEqual(tt.wantQueue, queue) {
				t.Fatalf("workspace queue expected to be %#v, but got %#v", tt.wantQueue, queue)
			}
		})
	}
}
