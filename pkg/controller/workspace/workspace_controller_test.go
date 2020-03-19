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
		name      string
		workspace *terraformv1alpha1.Workspace
		commands  []*terraformv1alpha1.Command
		wantQueue []string
	}{
		{
			name:      "No commands",
			workspace: &workspace,
			commands:  []*terraformv1alpha1.Command{},
			wantQueue: []string{},
		},
		{
			name:      "Single command",
			workspace: &workspace,
			commands:  []*terraformv1alpha1.Command{&command1},
			wantQueue: []string{"command1"},
		},
		{
			name:      "Two commands",
			workspace: &workspace,
			commands:  []*terraformv1alpha1.Command{&command1, &command2},
			wantQueue: []string{"command1", "command2"},
		},
		{
			name:      "Three commands",
			workspace: &workspace,
			commands:  []*terraformv1alpha1.Command{&command1, &command2, &command3},
			wantQueue: []string{"command1", "command2", "command3"},
		},
	}
	s := scheme.Scheme
	s.AddKnownTypes(terraformv1alpha1.SchemeGroupVersion, &terraformv1alpha1.Workspace{}, &terraformv1alpha1.CommandList{}, &terraformv1alpha1.Command{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := tt.workspace.DeepCopy()
			objs := []runtime.Object{workspace}
			for _, c := range tt.commands {
				objs = append(objs, c.DeepCopy())
			}

			cl := fake.NewFakeClientWithScheme(s, objs...)

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
			if res.Requeue {
				t.Error("didn't expect reconcile to requeue")
			}

			pvc := &corev1.PersistentVolumeClaim{}
			// I'm not sure if pvc will have been created just yet...
			err = r.client.Get(context.TODO(), req.NamespacedName, pvc)
			if err != nil {
				t.Fatalf("get pvc: (%v)", err)
			}

			if res.Requeue {
				t.Error("didn't expect reconcile to requeue")
			}

			err = r.client.Get(context.TODO(), req.NamespacedName, workspace)
			if err != nil {
				t.Fatalf("get ws: (%v)", err)
			}

			queue := workspace.Status.Queue
			if reflect.DeepEqual(tt.wantQueue, queue) {
				t.Fatalf("workspace queue expected to be %+v, but got %+v", tt.wantQueue, queue)
			}
		})
	}
}
