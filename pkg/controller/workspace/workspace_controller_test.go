package workspace

import (
	"context"
	"reflect"
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/status"

	terraformv1alpha1 "github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var workspaceEmptyQueue = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
}

var workspaceWithQueue = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Status: terraformv1alpha1.WorkspaceStatus{
		Queue: []string{
			"command-1",
		},
	},
}

var command1 = terraformv1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-1",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
	},
}

var command2 = terraformv1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-2",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
	},
}

var completedCommand = terraformv1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-3",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
	},
	Status: terraformv1alpha1.CommandStatus{
		Conditions: status.Conditions{
			"Completed": status.Condition{
				Type:   status.ConditionType("Completed"),
				Status: corev1.ConditionTrue,
			},
		},
	},
}

var commandWithNonExistantWorkspace = terraformv1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-with-nonexistant-workspace",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-does-not-exist",
		},
	},
}

func TestReconcileWorkspace(t *testing.T) {
	tests := []struct {
		name        string
		workspace   *terraformv1alpha1.Workspace
		objs        []runtime.Object
		wantQueue   []string
		wantRequeue bool
	}{
		{
			name:      "No commands",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&commandWithNonExistantWorkspace),
			},
			wantQueue:   []string{},
			wantRequeue: false,
		},
		{
			name:      "Single command",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&command1),
			},
			wantQueue:   []string{"command-1"},
			wantRequeue: false,
		},
		{
			name:      "Two commands",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&command1),
				runtime.Object(&command2),
			},
			wantQueue:   []string{"command-1", "command-2"},
			wantRequeue: false,
		},
		{
			name:      "Existing queue",
			workspace: &workspaceWithQueue,
			objs: []runtime.Object{
				runtime.Object(&command1),
				runtime.Object(&command2),
			},
			wantQueue:   []string{"command-1", "command-2"},
			wantRequeue: false,
		},
		{
			name:      "Completed command",
			workspace: &workspaceEmptyQueue,
			objs: []runtime.Object{
				runtime.Object(&completedCommand),
				runtime.Object(&command1),
				runtime.Object(&command2),
			},
			wantQueue:   []string{"command-1", "command-2"},
			wantRequeue: false,
		},
	}
	s := scheme.Scheme
	s.AddKnownTypes(terraformv1alpha1.SchemeGroupVersion, &terraformv1alpha1.Workspace{}, &terraformv1alpha1.CommandList{}, &terraformv1alpha1.Command{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			err = r.client.Get(context.TODO(), req.NamespacedName, tt.workspace)
			if err != nil {
				t.Fatalf("get ws: (%v)", err)
			}

			queue := tt.workspace.Status.Queue
			if !reflect.DeepEqual(tt.wantQueue, queue) {
				t.Fatalf("workspace queue expected to be %+v, but got %+v", tt.wantQueue, queue)
			}
		})
	}
}
