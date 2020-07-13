package command

import (
	"context"
	"testing"

	"github.com/leg100/stok/pkg/apis"
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
	Status: corev1.PodStatus{
		Phase: corev1.PodPending,
	},
}

var podRunningAndReady = corev1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "plan-1",
		Namespace: "operator-test",
	},
	Status: corev1.PodStatus{
		Phase: corev1.PodRunning,
		Conditions: []corev1.PodCondition{
			{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
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
		name                     string
		annotations              map[string]string
		conditions               status.Conditions
		objs                     []runtime.Object
		wantAttachableCondition  corev1.ConditionStatus
		wantClientReadyCondition corev1.ConditionStatus
		wantCompletedCondition   corev1.ConditionStatus
		wantRequeue              bool
		wantGoogleCredentials    bool
	}{
		{
			name: "Missing workspace",
			objs: []runtime.Object{
				runtime.Object(&secret),
			},
			wantAttachableCondition:  corev1.ConditionUnknown,
			wantClientReadyCondition: corev1.ConditionUnknown,
			wantCompletedCondition:   corev1.ConditionTrue,
		},
		{
			name: "Create pod",
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
			},
			wantAttachableCondition:  corev1.ConditionUnknown,
			wantClientReadyCondition: corev1.ConditionUnknown,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantGoogleCredentials:    true,
		},
		{
			name: "Client has set annotation",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&pod),
			},
			wantClientReadyCondition: corev1.ConditionTrue,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantAttachableCondition:  corev1.ConditionUnknown,
		},
		{
			name: "Successfully completed pod",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&successfullyCompletedPod),
			},
			wantClientReadyCondition: corev1.ConditionTrue,
			wantAttachableCondition:  corev1.ConditionUnknown,
			wantCompletedCondition:   corev1.ConditionTrue,
		},
		{
			name: "Unenqueued command",
			objs: []runtime.Object{
				runtime.Object(&workspaceEmptyQueue),
				runtime.Object(&pod),
			},
			wantClientReadyCondition: corev1.ConditionUnknown,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantAttachableCondition:  corev1.ConditionFalse,
		},
		{
			name: "Waiting in queue",
			objs: []runtime.Object{
				runtime.Object(&workspaceBackOfQueue),
				runtime.Object(&pod),
			},
			wantClientReadyCondition: corev1.ConditionUnknown,
			wantAttachableCondition:  corev1.ConditionFalse,
			wantCompletedCondition:   corev1.ConditionUnknown,
		},
		{
			name: "Command at front of queue with pod running and ready",
			annotations: map[string]string{
				"stok.goalspike.com/client": "Ready",
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
				runtime.Object(&podRunningAndReady),
			},
			wantClientReadyCondition: corev1.ConditionTrue,
			wantCompletedCondition:   corev1.ConditionUnknown,
			wantAttachableCondition:  corev1.ConditionTrue,
		},
	}
	s := scheme.Scheme
	apis.AddToScheme(s)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &v1alpha1.Plan{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Plan",
					APIVersion: "stok.goalspike.com/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
					Labels: map[string]string{
						"app":       "stok",
						"workspace": "workspace-1",
					},
				},
			}

			plan.SetConditions(tt.conditions)
			plan.SetAnnotations(tt.annotations)

			objs := append(tt.objs, runtime.Object(plan))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      plan.GetName(),
					Namespace: plan.GetNamespace(),
				},
			}
			r := &CommandReconciler{client: cl, scheme: s, gvk: plan.GroupVersionKind(), entrypoint: []string{"terraform", "plan"}, plural: "plans"}

			res, err := r.Reconcile(req)
			if err != nil {
				t.Fatal(err)
			}

			if tt.wantRequeue {
				if res.Requeue {
					res, err = r.Reconcile(req)
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

			err = cl.Get(context.TODO(), req.NamespacedName, plan)
			if err != nil {
				t.Fatalf("get command: (%v)", err)
			}

			if tt.wantGoogleCredentials {
				want := "/credentials/google-credentials.json"
				got, ok := getEnvValueForName(&pod.Spec.Containers[0], "GOOGLE_APPLICATION_CREDENTIALS")
				if !ok {
					t.Errorf("Could not find env var with name GOOGLE_APPLICATION_CREDENTIALS")
				}
				if want != got {
					t.Errorf("want %s got %s", want, got)
				}
			}

			assertCondition(t, &plan.CommandStatus, v1alpha1.ConditionCompleted, tt.wantCompletedCondition)
			assertCondition(t, &plan.CommandStatus, v1alpha1.ConditionAttachable, tt.wantAttachableCondition)
			assertCondition(t, &plan.CommandStatus, v1alpha1.ConditionClientReady, tt.wantClientReadyCondition)
		})
	}
}

func getEnvValueForName(container *corev1.Container, name string) (string, bool) {
	for _, env := range container.Env {
		if env.Name == name {
			return env.Value, true
		}
	}
	return "", false
}

func assertCondition(t *testing.T, cmdstatus *v1alpha1.CommandStatus, ctype status.ConditionType, want corev1.ConditionStatus) {
	if cmdstatus.Conditions.IsUnknownFor(ctype) && want != corev1.ConditionUnknown ||
		cmdstatus.Conditions.IsTrueFor(ctype) && want != corev1.ConditionTrue ||
		cmdstatus.Conditions.IsFalseFor(ctype) && want != corev1.ConditionFalse {

		t.Errorf("expected %s status to be %v, got '%v'", ctype, want, cmdstatus.Conditions.GetCondition(ctype))
	}
}
