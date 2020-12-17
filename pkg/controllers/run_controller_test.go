package controllers

import (
	"context"
	"testing"

	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/scheme"
	"github.com/leg100/etok/pkg/testobj"
	"github.com/leg100/etok/pkg/testutil"
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
		},
		{
			name: "Owned",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhasePending)),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1"),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, "Workspace", run.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", run.OwnerReferences[0].Name)
			},
		},
		{
			name: "Unqueued",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithSecret("secret-1")),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhasePending, run.Phase)
			},
		},
		{
			name: "Queued",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-0", "plan-1")),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseQueued, run.Phase)
			},
		},
		{
			name: "Provisioning",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithSecret("secret-1"), testobj.WithQueue("plan-1")),
				testobj.RunPod("operator-test", "plan-1", testobj.WithPhase(corev1.PodPending)),
			},
			assertions: func(run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseProvisioning, run.Phase)
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
				assert.Equal(t, v1alpha1.RunPhaseRunning, run.Phase)
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
				assert.Equal(t, v1alpha1.RunPhaseCompleted, run.Phase)
			},
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			objs := append(tt.objs, runtime.Object(tt.run))
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.run.Name,
					Namespace: tt.run.Namespace,
				},
			}

			_, err := NewRunReconciler(cl, "a.b.c/d:v1").Reconcile(req)
			t.CheckError(tt.reconcileError, err)

			run := &v1alpha1.Run{}
			require.NoError(t, cl.Get(context.TODO(), req.NamespacedName, run))

			if tt.assertions != nil {
				tt.assertions(run)
			}
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
			name: "Image name",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
			},
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, "a.b.c/d:v1", pod.Spec.Containers[0].Image)
			},
		},
		{
			name: "Sets container args",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithArgs("-out", "plan.out")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithQueue("plan-1")),
			},
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t, []string{"--", "-out", "plan.out"}, pod.Spec.Containers[0].Args)
			},
		},
	}
	for _, tt := range podTests {
		t.Run(tt.name, func(t *testing.T) {
			objs := append(tt.objs, runtime.Object(tt.run))
			cl := fake.NewFakeClientWithScheme(scheme.Scheme, objs...)

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.run.Name,
					Namespace: tt.run.Namespace,
				},
			}

			_, err := NewRunReconciler(cl, "a.b.c/d:v1").Reconcile(req)
			assert.NoError(t, err)

			pod := &corev1.Pod{}
			require.NoError(t, cl.Get(context.TODO(), req.NamespacedName, pod))

			tt.assertions(pod)
		})
	}
}
