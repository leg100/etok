package command

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
	Spec: terraformv1alpha1.CommandSpec{
		Args: []string{"version"},
	},
}

func TestReconcileCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   *terraformv1alpha1.Command
		workspace *terraformv1alpha1.Workspace
	}{
		{
			name:      "No commands",
			command:   &command1,
			workspace: &workspace,
		},
	}
	s := scheme.Scheme
	s.AddKnownTypes(terraformv1alpha1.SchemeGroupVersion, &terraformv1alpha1.Workspace{}, &terraformv1alpha1.CommandList{}, &terraformv1alpha1.Command{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := tt.workspace.DeepCopy()
			command := tt.command.DeepCopy()
			objs := []runtime.Object{workspace, command}

			cl := fake.NewFakeClientWithScheme(s, objs...)

			r := &ReconcileCommand{client: cl, scheme: s}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      command.GetName(),
					Namespace: command.GetNamespace(),
				},
			}
			res, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("reconcile: (%v)", err)
			}
			if res.Requeue {
				t.Error("didn't expect reconcile to requeue")
			}

			pod := &corev1.Pod{}
			err = r.client.Get(context.TODO(), req.NamespacedName, pod)
			if err != nil {
				t.Fatalf("get pod: (%v)", err)
			}

			expectedArgs := []string{"version"}
			if !reflect.DeepEqual(pod.Spec.Containers[0].Args, expectedArgs) {
				t.Fatalf("expected args, %+v, but got %+v", expectedArgs, pod.Spec.Containers[0].Args)
			}

			if res.Requeue {
				t.Error("didn't expect reconcile to requeue")
			}
		})
	}
}
