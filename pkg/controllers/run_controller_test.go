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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestRunReconciler(t *testing.T) {
	tests := []struct {
		name                string
		run                 *v1alpha1.Run
		objs                []runtime.Object
		runAssertions       func(*testutil.T, *v1alpha1.Run)
		podAssertions       func(*testutil.T, *corev1.Pod)
		configMapAssertions func(*testutil.T, *corev1.ConfigMap)
		reconcileError      bool
	}{
		{
			name: "Missing workspace",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				if assert.NotNil(t, run.Conditions) {
					failed := meta.FindStatusCondition(run.Conditions, v1alpha1.RunFailedCondition)
					if assert.NotNil(t, failed) {
						assert.Equal(t, metav1.ConditionTrue, failed.Status)
						assert.Equal(t, v1alpha1.WorkspaceNotFoundReason, failed.Reason)
					}
				}
			},
		},
		{
			name: "Owned",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithRunPhase(v1alpha1.RunPhaseWaiting)),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1"),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, "Workspace", run.OwnerReferences[0].Kind)
				assert.Equal(t, "workspace-1", run.OwnerReferences[0].Name)
			},
		},
		{
			name: "Plan is unqueued and its pod is immediately provisioned",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithSecret("secret-1")),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseProvisioning, run.Phase)
			},
		},
		{
			name: "Queued",
			run:  testobj.Run("operator-test", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("apply-0", "apply-1")),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseQueued, run.Phase)
			},
		},
		{
			name: "Provisioning run at front of queue",
			run:  testobj.Run("operator-test", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithSecret("secret-1"), testobj.WithCombinedQueue("apply-1")),
				testobj.RunPod("operator-test", "apply-1", testobj.WithPhase(corev1.PodPending)),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseProvisioning, run.Phase)
			},
		},
		{
			name: "Running unqueued plan",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1"),
				testobj.RunPod("operator-test", "plan-1"),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseRunning, run.Phase)
			},
		},
		{
			name: "Running apply at front of queue",
			run:  testobj.Run("operator-test", "apply-1", "apply", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("apply-1")),
				testobj.RunPod("operator-test", "apply-1"),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, v1alpha1.RunPhaseRunning, run.Phase)
			},
		},
		{
			name: "Completed",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("plan-1")),
				testobj.RunPod("operator-test", "plan-1", testobj.WithPhase(corev1.PodSucceeded)),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.True(t, meta.IsStatusConditionTrue(run.Conditions, v1alpha1.RunCompleteCondition))
			},
		},
		{
			name: "Creates pod",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("plan-1")),
			},
			podAssertions: func(t *testutil.T, pod *corev1.Pod) {
				assert.NotEqual(t, &corev1.Pod{}, pod)
			},
		},
		{
			name: "Image name",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("plan-1")),
			},
			podAssertions: func(t *testutil.T, pod *corev1.Pod) {
				assert.Equal(t, "a.b.c/d:v1", pod.Spec.Containers[0].Image)
			},
		},
		{
			name: "Sets container args",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1"), testobj.WithArgs("-out", "plan.out")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("plan-1")),
			},
			podAssertions: func(t *testutil.T, pod *corev1.Pod) {
				assert.Equal(t, []string{"--", "-out", "plan.out"}, pod.Spec.Containers[0].Args)
			},
		},
		{
			name: "Run owns config map",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithCombinedQueue("plan-1")),
				testobj.ConfigMap("operator-test", "plan-1"),
			},
			configMapAssertions: func(t *testutil.T, archive *corev1.ConfigMap) {
				assert.Equal(t, "Run", archive.OwnerReferences[0].Kind)
				assert.Equal(t, "plan-1", archive.OwnerReferences[0].Name)
			},
		},
		{
			name: "Exit code recorded in status",
			run:  testobj.Run("operator-test", "plan-1", "plan", testobj.WithWorkspace("workspace-1")),
			objs: []runtime.Object{
				testobj.Workspace("operator-test", "workspace-1", testobj.WithSecret("secret-1")),
				testobj.RunPod("operator-test", "plan-1", testobj.WithPhase(corev1.PodSucceeded), testobj.WithRunnerExitCode(5)),
			},
			runAssertions: func(t *testutil.T, run *v1alpha1.Run) {
				assert.Equal(t, 5, *run.RunStatus.ExitCode)
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

			_, err := NewRunReconciler(cl, "a.b.c/d:v1").Reconcile(context.Background(), req)
			t.CheckError(tt.reconcileError, err)

			if tt.runAssertions != nil {
				var run v1alpha1.Run
				require.NoError(t, cl.Get(context.TODO(), req.NamespacedName, &run))

				tt.runAssertions(t, &run)
			}

			if tt.podAssertions != nil {
				var pod corev1.Pod
				require.NoError(t, cl.Get(context.TODO(), req.NamespacedName, &pod))

				tt.podAssertions(t, &pod)
			}

			if tt.configMapAssertions != nil {
				var archive corev1.ConfigMap
				require.NoError(t, cl.Get(context.TODO(), req.NamespacedName, &archive))

				tt.configMapAssertions(t, &archive)
			}
		})
	}
}
