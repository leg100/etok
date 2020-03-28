package workspace

import (
	"context"
	"reflect"
	"testing"

	terraformv1alpha1 "github.com/leg100/terraform-operator/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var workspace = terraformv1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
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

var command3 = terraformv1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-3",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
	},
}

func TestReconcileWorkspace(t *testing.T) {
	tests := []struct {
		name        string
		objs        []runtime.Object
		wantQueue   []string
		wantRequeue bool
	}{
		{
			name: "No commands",
			objs: []runtime.Object{
				runtime.Object(&workspace),
			},
			wantRequeue: false,
		},
		{
			name: "Single command",
			objs: []runtime.Object{
				runtime.Object(&workspace),
				runtime.Object(&command1),
			},
			wantQueue:   []string{"command-1"},
			wantRequeue: false,
		},
		{
			name: "Two commands",
			objs: []runtime.Object{
				runtime.Object(&workspace),
				runtime.Object(&command1),
				runtime.Object(&command2),
			},
			wantQueue:   []string{"command-1", "command-2"},
			wantRequeue: false,
		},
		{
			name: "Three commands",
			objs: []runtime.Object{
				runtime.Object(&workspace),
				runtime.Object(&command1),
				runtime.Object(&command2),
				runtime.Object(&command3),
			},
			wantQueue:   []string{"command-1", "command-2", "command-3"},
			wantRequeue: false,
		},
	}
	s := scheme.Scheme
	s.AddKnownTypes(terraformv1alpha1.SchemeGroupVersion, &terraformv1alpha1.Workspace{}, &terraformv1alpha1.CommandList{}, &terraformv1alpha1.Command{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewFakeClientWithScheme(s, tt.objs...)

			r := &ReconcileWorkspace{client: cl, scheme: s}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      workspace.GetName(),
					Namespace: workspace.GetNamespace(),
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

			workspace := terraformv1alpha1.Workspace{}
			err = r.client.Get(context.TODO(), req.NamespacedName, &workspace)
			if err != nil {
				t.Fatalf("get ws: (%v)", err)
			}

			queue := workspace.Status.Queue
			if !reflect.DeepEqual(tt.wantQueue, queue) {
				t.Fatalf("workspace queue expected to be %+v, but got %+v", tt.wantQueue, queue)
			}
		})
	}
}
