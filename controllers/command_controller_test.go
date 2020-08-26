package controllers

import (
	"context"
	"testing"

	"github.com/leg100/stok/api/command"
	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestCommandReconciler(t *testing.T) {
	plan1 := v1alpha1.Plan{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "stok.goalspike.com/v1alpha1",
			Kind:       "Plan",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plan-1",
			Namespace: "operator-test",
		},
		CommandSpec: v1alpha1.CommandSpec{
			Workspace: "workspace-1",
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

	tests := []struct {
		name           string
		cmd            command.Interface
		objs           []runtime.Object
		assertions     func(cmd command.Interface)
		reconcileError bool
	}{
		{
			name:           "Missing workspace",
			cmd:            &plan1,
			reconcileError: true,
			assertions:     func(cmd command.Interface) {},
		},
		{
			name: "Pending",
			cmd:  &plan1,
			objs: []runtime.Object{
				runtime.Object(&workspaceEmptyQueue),
				runtime.Object(&pod),
			},
			assertions: func(cmd command.Interface) {
				assert.Equal(t, v1alpha1.CommandPhasePending, cmd.GetPhase())
			},
		},
		{
			name: "Queued",
			cmd:  &plan1,
			objs: []runtime.Object{
				runtime.Object(&workspaceBackOfQueue),
				runtime.Object(&pod),
			},
			assertions: func(cmd command.Interface) {
				assert.Equal(t, v1alpha1.CommandPhaseQueued, cmd.GetPhase())
			},
		},
		{
			name: "Synchronising",
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "plan-1",
					Namespace:   "operator-test",
					Annotations: map[string]string{v1alpha1.WaitAnnotationKey: "true"},
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&podRunningAndReady),
			},
			assertions: func(cmd command.Interface) {
				assert.Equal(t, v1alpha1.CommandPhaseSync, cmd.GetPhase())
			},
		},
		{
			name: "Completed",
			cmd:  &plan1,
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&successfullyCompletedPod),
			},
			assertions: func(cmd command.Interface) {
				assert.Equal(t, v1alpha1.CommandPhaseCompleted, cmd.GetPhase())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			objs := append(tt.objs, runtime.Object(tt.cmd))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.cmd.GetName(),
					Namespace: tt.cmd.GetNamespace(),
				},
			}

			kind, _ := GetKindFromObject(s, tt.cmd)

			_, err := NewCommandReconciler(cl, kind, "a.b.c/d:v1").Reconcile(req)
			if tt.reconcileError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			cmd, _ := command.NewCommandFromGVK(s, v1alpha1.SchemeGroupVersion.WithKind(kind))
			err = cl.Get(context.TODO(), req.NamespacedName, cmd)
			require.NoError(t, err)

			tt.assertions(cmd)
		})
	}

	podTests := []struct {
		name       string
		cmd        command.Interface
		objs       []runtime.Object
		assertions func(pod *corev1.Pod)
	}{
		{
			name: "Creates pod",
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
			},
			assertions: func(pod *corev1.Pod) {
				assert.NotEqual(t, &corev1.Pod{}, pod)
			},
		},
		{
			name: "Sets google credentials",
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&secret),
			},
			assertions: func(pod *corev1.Pod) {
				want := "/credentials/google-credentials.json"
				got, ok := getEnvValueForName(&pod.Spec.Containers[0], "GOOGLE_APPLICATION_CREDENTIALS")
				if !ok {
					t.Errorf("Could not find env var with name GOOGLE_APPLICATION_CREDENTIALS")
				} else {
					assert.Equal(t, want, got)
				}
			},
		},
		{
			name: "Image name",
			cmd: &v1alpha1.Plan{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				CommandSpec: v1alpha1.CommandSpec{
					Workspace: "workspace-1",
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
			},
			assertions: func(pod *corev1.Pod) {
				require.Equal(t, "a.b.c/d:v1", pod.Spec.Containers[0].Image)
			},
		},
	}
	for _, tt := range podTests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			objs := append(tt.objs, runtime.Object(tt.cmd))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.cmd.GetName(),
					Namespace: tt.cmd.GetNamespace(),
				},
			}

			kind, _ := GetKindFromObject(s, tt.cmd)
			_, err := NewCommandReconciler(cl, kind, "a.b.c/d:v1").Reconcile(req)
			assert.NoError(t, err)

			pod := &corev1.Pod{}
			_ = cl.Get(context.TODO(), req.NamespacedName, pod)

			tt.assertions(pod)
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
