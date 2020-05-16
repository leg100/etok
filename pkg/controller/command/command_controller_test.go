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

// conditions lifecyle:
// 1. check for workspace, if not found set WorkspaceReady to false
// 2. check if client has set annotation, if set set ClientReady to true
// 3. check pod, if completed successfully or failed, set Complete to true
// 4. check workspace queue
//  a. if unenqueued, set WorkspaceReady to false, reason unenqueued
//  b. if queue pos is >0, set WorkspaceReady to false, reason queued
//  b. if queue pos is 0, set WorkspaceReady to true, reason active

var plan = v1alpha1.Plan{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "plan-1",
		Namespace: "operator-test",
		Labels: map[string]string{
			"workspace": "workspace-1",
		},
	},
	CommandSpec: v1alpha1.CommandSpec{
		Args: []string{"version"},
	},
}

var secret = corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "secret-1",
		Namespace: "operator-test",
	},
	StringData: map[string]string{
		"google_application_credentials.json": "abc",
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
		Queue: []string{"plan-1"},
	},
}

var workspaceBackOfQueue = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "workspace-1",
		Namespace: "operator-test",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName: "secret-1",
	},
	Status: v1alpha1.WorkspaceStatus{
		Queue: []string{"plan-0", "plan-1"},
	},
}

var pod = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "plan-1",
		Namespace: "operator-test",
	},
}

var successfullyCompletedPod = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "plan-1",
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
		name                        string
		annotations                 map[string]string
		conditions                  status.Conditions
		objs                        []runtime.Object
		wantClientReadyCondition    corev1.ConditionStatus
		wantCompletedCondition      corev1.ConditionStatus
		wantWorkspaceReadyCondition corev1.ConditionStatus
		wantRequeue                 bool
		wantGoogleCredentials       bool
	}{
		{
			name: "Missing workspace",
			objs: []runtime.Object{
				runtime.Object(&secret),
			},
			wantWorkspaceReadyCondition: corev1.ConditionFalse,
			wantClientReadyCondition:    corev1.ConditionUnknown,
			wantCompletedCondition:      corev1.ConditionUnknown,
			wantRequeue:                 false,
		},
		{
			name: "Create pod",
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
			},
			wantWorkspaceReadyCondition: corev1.ConditionTrue,
			wantClientReadyCondition:    corev1.ConditionUnknown,
			wantCompletedCondition:      corev1.ConditionUnknown,
			wantRequeue:                 true,
			wantGoogleCredentials:       true,
		},
		{
			name: "Client has set annotation",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
				runtime.Object(&pod),
			},
			wantClientReadyCondition:    corev1.ConditionTrue,
			wantCompletedCondition:      corev1.ConditionUnknown,
			wantWorkspaceReadyCondition: corev1.ConditionUnknown,
			wantRequeue:                 false,
		},
		{
			name: "Successfully completed pod",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			conditions: status.Conditions{
				{
					Type:    status.ConditionType("ClientReady"),
					Reason:  status.ConditionReason("ClientReceivingLogs"),
					Message: "Logs are being streamed to the client",
					Status:  corev1.ConditionTrue,
				},
				{
					Type:    status.ConditionType("WorkspaceReady"),
					Reason:  status.ConditionReason("Active"),
					Message: "Front of workspace queue",
					Status:  corev1.ConditionTrue,
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
				runtime.Object(&successfullyCompletedPod),
			},
			wantClientReadyCondition:    corev1.ConditionTrue,
			wantWorkspaceReadyCondition: corev1.ConditionTrue,
			wantCompletedCondition:      corev1.ConditionTrue,
			wantRequeue:                 false,
		},
		{
			name: "Unenqueued command",
			objs: []runtime.Object{
				runtime.Object(&workspaceEmptyQueue),
				runtime.Object(&secret),
				runtime.Object(&pod),
			},
			wantClientReadyCondition:    corev1.ConditionUnknown,
			wantCompletedCondition:      corev1.ConditionUnknown,
			wantWorkspaceReadyCondition: corev1.ConditionFalse,
			wantRequeue:                 false,
		},
		{
			name: "Waiting in queue",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceBackOfQueue),
				runtime.Object(&secret),
				runtime.Object(&pod),
			},
			conditions: status.Conditions{
				{
					Type:    status.ConditionType("ClientReady"),
					Reason:  status.ConditionReason("ClientReceivingLogs"),
					Message: "Logs are being streamed to the client",
					Status:  corev1.ConditionTrue,
				},
			},
			wantClientReadyCondition:    corev1.ConditionTrue,
			wantWorkspaceReadyCondition: corev1.ConditionFalse,
			wantCompletedCondition:      corev1.ConditionUnknown,
			wantRequeue:                 false,
		},
		{
			name: "Command at front of queue",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
				runtime.Object(&pod),
			},
			conditions: status.Conditions{
				{
					Type:    status.ConditionType("ClientReady"),
					Reason:  status.ConditionReason("ClientReceivingLogs"),
					Message: "Logs are being streamed to the client",
					Status:  corev1.ConditionTrue,
				},
			},
			wantClientReadyCondition:    corev1.ConditionTrue,
			wantCompletedCondition:      corev1.ConditionUnknown,
			wantWorkspaceReadyCondition: corev1.ConditionTrue,
			wantRequeue:                 false,
		},
	}
	s := scheme.Scheme
	crdapi.AddToScheme(s)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan.SetConditions(tt.conditions)
			plan.SetAnnotations(tt.annotations)

			objs := append(tt.objs, runtime.Object(&plan))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      plan.GetName(),
					Namespace: plan.GetNamespace(),
				},
			}

			res, err := Reconcile(cl, s, req, &plan, "plan")
			if err != nil {
				t.Fatalf("reconcile: (%v)", err)
			}

			if tt.wantRequeue {
				if res.Requeue {
					res, err = Reconcile(cl, s, req, &plan, "plan")
					if err != nil {
						t.Fatalf("requeued reconcile: (%v)", err)
					}
				} else {
					t.Error("want requeue got no requeue")
				}
			}

			pod := &corev1.Pod{}
			err = cl.Get(context.TODO(), req.NamespacedName, pod)
			if err != nil && !errors.IsNotFound(err) {
				t.Fatalf("error fetching pod %v", err)
			}

			err = cl.Get(context.TODO(), req.NamespacedName, &plan)
			if err != nil {
				t.Fatalf("get command: (%v)", err)
			}

			if tt.wantGoogleCredentials {
				want := "/credentials/google-credentials.json"
				got := pod.Spec.Containers[0].Env[0].Value
				if want != got {
					t.Errorf("want %s got %s", want, got)
				}
			}

			assertCondition(t, &plan, "Completed", tt.wantCompletedCondition)
			assertCondition(t, &plan, "WorkspaceReady", tt.wantWorkspaceReadyCondition)
			assertCondition(t, &plan, "ClientReady", tt.wantClientReadyCondition)
		})
	}
}

func assertCondition(t *testing.T, command Command, conditionType string, want corev1.ConditionStatus) {
	if command.GetConditions().IsUnknownFor(status.ConditionType(conditionType)) && want != corev1.ConditionUnknown ||
		command.GetConditions().IsTrueFor(status.ConditionType(conditionType)) && want != corev1.ConditionTrue ||
		command.GetConditions().IsFalseFor(status.ConditionType(conditionType)) && want != corev1.ConditionFalse {

		t.Errorf("expected %s status to be %v, got '%v'", conditionType, want, command.GetConditions().GetCondition(status.ConditionType(conditionType)))
	}
}
