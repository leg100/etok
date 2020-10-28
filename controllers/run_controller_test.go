package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
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

func TestRunReconciler(t *testing.T) {
	plan1 := v1alpha1.Run{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "stok.goalspike.com/v1alpha1",
			Kind:       "Run",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plan-1",
			Namespace: "operator-test",
		},
		RunSpec: v1alpha1.RunSpec{
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
		run            *v1alpha1.Run
		objs           []runtime.Object
		assertions     func(run *v1alpha1.Run)
		reconcileError bool
	}{
		{
			name:           "Missing workspace",
			run:            &plan1,
			reconcileError: true,
			assertions:     func(run *v1alpha1.Run) {},
		},
		{
			name: "Pending",
			run:  &plan1,
			objs: []runtime.Object{
				runtime.Object(&workspaceEmptyQueue),
				runtime.Object(&pod),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhasePending, run.GetPhase())
			},
		},
		{
			name: "Queued",
			run:  &plan1,
			objs: []runtime.Object{
				runtime.Object(&workspaceBackOfQueue),
				runtime.Object(&pod),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseQueued, run.GetPhase())
			},
		},
		{
			name: "Running",
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
					Workspace: "workspace-1",
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&podRunningAndReady),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseRunning, run.GetPhase())
			},
		},
		{
			name: "Completed",
			run:  &plan1,
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
				runtime.Object(&successfullyCompletedPod),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseCompleted, run.GetPhase())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			objs := append(tt.objs, runtime.Object(tt.run))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.run.GetName(),
					Namespace: tt.run.GetNamespace(),
				},
			}

			_, err := NewRunReconciler(cl, "a.b.c/d:v1").Reconcile(req)
			if tt.reconcileError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			run := &v1alpha1.Run{}
			err = cl.Get(context.TODO(), req.NamespacedName, run)
			require.NoError(t, err)

			tt.assertions(run)
		})
	}

	podTests := []struct {
		name       string
		run        *v1alpha1.Run
		objs       []runtime.Object
		assertions func(pod *corev1.Pod)
	}{
		{
			name: "Creates pod",
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
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
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
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
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
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
		{
			name: "Sets container args",
			run: &v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "plan-1",
					Namespace: "operator-test",
				},
				RunSpec: v1alpha1.RunSpec{
					Command:   "plan",
					Workspace: "workspace-1",
				},
			},
			objs: []runtime.Object{
				runtime.Object(&workspaceQueueOfOne),
			},
			assertions: func(pod *corev1.Pod) {
				require.Equal(t, []string{"--", "terraform", "plan"}, pod.Spec.Containers[0].Args)
			},
		},
	}
	for _, tt := range podTests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheme.Scheme
			objs := append(tt.objs, runtime.Object(tt.run))
			cl := fake.NewFakeClientWithScheme(s, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.run.GetName(),
					Namespace: tt.run.GetNamespace(),
				},
			}

			_, err := NewRunReconciler(cl, "a.b.c/d:v1").Reconcile(req)
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
