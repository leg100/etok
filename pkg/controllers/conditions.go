package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	restoreFailure = workspaceConditionSetterFactory(v1alpha1.RestoreFailureCondition, metav1.ConditionTrue)
	restoreOK      = workspaceConditionSetterFactory(v1alpha1.RestoreFailureCondition, metav1.ConditionFalse)

	backupFailure = workspaceConditionSetterFactory(v1alpha1.BackupFailureCondition, metav1.ConditionTrue)
	backupOK      = workspaceConditionSetterFactory(v1alpha1.BackupFailureCondition, metav1.ConditionFalse)

	cacheFailure = workspaceConditionSetterFactory(v1alpha1.CacheFailureCondition, metav1.ConditionTrue)
	cacheUnknown = workspaceConditionSetterFactory(v1alpha1.CacheFailureCondition, metav1.ConditionUnknown)
	cacheOK      = workspaceConditionSetterFactory(v1alpha1.CacheFailureCondition, metav1.ConditionFalse)

	podFailure = workspaceConditionSetterFactory(v1alpha1.PodFailureCondition, metav1.ConditionTrue)
	podUnknown = workspaceConditionSetterFactory(v1alpha1.PodFailureCondition, metav1.ConditionUnknown)
	podOK      = workspaceConditionSetterFactory(v1alpha1.PodFailureCondition, metav1.ConditionFalse)
)

type workspaceConditionSetter func(*v1alpha1.Workspace, string, string)

func workspaceConditionSetterFactory(condType string, status metav1.ConditionStatus) workspaceConditionSetter {
	return func(ws *v1alpha1.Workspace, reason, message string) {
		meta.SetStatusCondition(&ws.Status.Conditions, metav1.Condition{
			Type:    condType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
	}
}
