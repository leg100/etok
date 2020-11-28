package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestRunReconciler(t *testing.T) {
	tests := []struct {
		name           string
		run            *v1alpha1.Run
		objs           []runtime.Object
		assertions     func(run *v1alpha1.Run)
		reconcileError bool
	}{
		{
			name:           "Missing workspace",
			run:            testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			reconcileError: true,
			assertions:     func(run *v1alpha1.Run) {},
		},
		{
			name: "Pending",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithSecret("secret-1")),
				testobj.RunPod("operator-test", "plan-1", testobj.WithPhase(corev1.PodPending)),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhasePending, run.GetPhase())
			},
		},
		{
			name: "Queued",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-0", "plan-1")),
				testobj.RunPod("operator-test", "plan-1", testobj.WithPhase(corev1.PodPending)),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseQueued, run.GetPhase())
			},
		},
		{
			name: "Running",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
				testobj.RunPod("operator-test", "plan-1"),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseRunning, run.GetPhase())
			},
		},
		{
			name: "Completed",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
				testobj.RunPod("operator-test", "plan-1", testobj.WithPhase(corev1.PodSucceeded)),
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
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
			},
			assertions: func(pod *corev1.Pod) {
				assert.NotEqual(t, &corev1.Pod{}, pod)
			},
		},
		{
			name: "Sets google credentials",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1"), testobj.WithSecret("secret-1")),
				testobj.Secret("operator-test", "secret-1", testobj.WithStringData("google_application_credentials.json", "abc")),
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
			name: "Sets workspace environment variable",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
			},
			assertions: func(pod *corev1.Pod) {
				want := "operator-test-workspace-1"
				got, ok := getEnvValueForName(&pod.Spec.Containers[0], "TF_WORKSPACE")
				if !ok {
					t.Errorf("Could not find env var with name TF_WORKSPACE")
				} else {
					assert.Equal(t, want, got)
				}
			},
		},
		{
			name: "Image name",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
			},
			assertions: func(pod *corev1.Pod) {
				require.Equal(t, "a.b.c/d:v1", pod.Spec.Containers[0].Image)
			},
		},
		{
			name: "Sets container args",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
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
