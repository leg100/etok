package command

import (
	"context"
	"testing"

	crdapi "github.com/leg100/stok/pkg/apis"
	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var secret = corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "secret-1",
		Namespace: "operator-test",
	},
	StringData: map[string]string{
		"google_application_credentials.json": "abc",
	},
}

var command = v1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-1",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
	},
	Spec: v1alpha1.CommandSpec{
		Args: []string{"version"},
	},
}

var commandClientReady = v1alpha1.Command{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-1",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
		Annotations: map[string]string{
			"stok.goalspike.com/client": "Ready",
		},
	},
	Spec: v1alpha1.CommandSpec{
		Args: []string{"version"},
	},
}

var workspaceEmptyQueue = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName: "secret-1",
	},
}

var workspaceQueueOfOne = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName: "secret-1",
	},
	Status: v1alpha1.WorkspaceStatus{
		Queue: []string{"command-1"},
	},
}

var successfullyCompletedPod = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "command-1",
		Namespace: "operator-test",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodSucceeded,
	},
}

func newTrue() *bool {
	b := true
	return &b
}

func TestReconcileCommand(t *testing.T) {
	tests := []struct {
		name                     string
		command                  *v1alpha1.Command
		objs                     []runtime.Object
		wantPod                  bool
		wantClientReadyCondition corev1.ConditionStatus
		wantCompletedCondition   corev1.ConditionStatus
		wantRequeue              bool
	}{
		{
			name:    "Unqueued command",
			command: &command,
			objs: []runtime.Object{
				runtime.Object(&workspaceEmptyQueue),
				runtime.Object(&secret),
			},
			wantPod:                  false,
			wantClientReadyCondition: corev1.ConditionUnknown,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantRequeue:              false,
		},
		{
			name:    "Command at front of queue",
			command: &command,
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
			},
			wantPod:                  true,
			wantClientReadyCondition: corev1.ConditionUnknown,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantRequeue:              false,
		},
		{
			name:    "Command at front of queue and client is ready",
			command: &commandClientReady,
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
			},
			wantPod:                  true,
			wantClientReadyCondition: corev1.ConditionTrue,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantRequeue:              false,
		},
		{
			name:    "Successfully completed command",
			command: &commandClientReady,
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
				runtime.Object(&successfullyCompletedPod),
			},
			wantPod:                  true,
			wantClientReadyCondition: corev1.ConditionTrue,
			wantCompletedCondition:   corev1.ConditionTrue,
			wantRequeue:              false,
		},
	}
	s := scheme.Scheme
	crdapi.AddToScheme(s)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := append(tt.objs, runtime.Object(tt.command))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			r := &ReconcileCommand{client: cl, scheme: s}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.command.GetName(),
					Namespace: tt.command.GetNamespace(),
				},
			}
			res, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("reconcile: (%v)", err)
			}

			if tt.wantRequeue && !res.Requeue {
				t.Error("expected reconcile to requeue")
			}

			pod := &corev1.Pod{}
			err = r.client.Get(context.TODO(), req.NamespacedName, pod)
			if err != nil && !errors.IsNotFound(err) {
				t.Fatalf("error fetching pod %v", err)
			}
			if tt.wantPod && errors.IsNotFound(err) {
				t.Errorf("wanted pod but pod not found")
			}
			if !tt.wantPod && !errors.IsNotFound(err) {
				t.Errorf("did not want pod but pod found")
			}

			err = r.client.Get(context.TODO(), req.NamespacedName, tt.command)
			if err != nil {
				t.Fatalf("get command: (%v)", err)
			}

			assertCondition(t, tt.command, "Completed", tt.wantCompletedCondition)
		})
	}
}

func assertCondition(t *testing.T, command *v1alpha1.Command, conditionType string, want corev1.ConditionStatus) {
	if command.Status.Conditions.IsUnknownFor(status.ConditionType(conditionType)) && want != corev1.ConditionUnknown ||
		command.Status.Conditions.IsTrueFor(status.ConditionType(conditionType)) && want != corev1.ConditionTrue ||
		command.Status.Conditions.IsFalseFor(status.ConditionType(conditionType)) && want != corev1.ConditionFalse {

		t.Errorf("expected %s status to be %v, got %v", conditionType, want, command.Status.Conditions.GetCondition(status.ConditionType(conditionType)))
	}
}
