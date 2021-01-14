package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	restoreFailureCondition = "RestoreFailure"
	backupFailureCondition  = "BackupFailure"
	cacheFailureCondition   = "CacheFailure"
	podFailureCondition     = "PodFailure"

	clientCreateReason      = "ClientCreateFailed"
	bucketNotFoundReason    = "BucketNotFound"
	nothingToRestoreReason  = "NothingToRestore"
	unexpectedErrorReason   = "UnexpectedError"
	restoreSuccessfulReason = "RestoreSuccessful"
	backupSuccessfulReason  = "BackupSuccessful"
	cacheBoundReason        = "CacheBound"
	cacheLostReason         = "CacheLost"
	podCreatedReason        = "PodCreated"

	// Pending means whatever is being observed is reported to be progressing
	// towards a non-failure state.
	pendingReason = "Pending"
)

var (
	restoreFailure = workspaceConditionSetterFactory(restoreFailureCondition, metav1.ConditionTrue)
	restoreOK      = workspaceConditionSetterFactory(restoreFailureCondition, metav1.ConditionFalse)

	backupFailure = workspaceConditionSetterFactory(backupFailureCondition, metav1.ConditionTrue)
	backupOK      = workspaceConditionSetterFactory(backupFailureCondition, metav1.ConditionFalse)

	cacheFailure = workspaceConditionSetterFactory(cacheFailureCondition, metav1.ConditionTrue)
	cacheUnknown = workspaceConditionSetterFactory(cacheFailureCondition, metav1.ConditionUnknown)
	cacheOK      = workspaceConditionSetterFactory(cacheFailureCondition, metav1.ConditionFalse)

	podFailure = workspaceConditionSetterFactory(podFailureCondition, metav1.ConditionTrue)
	podUnknown = workspaceConditionSetterFactory(podFailureCondition, metav1.ConditionUnknown)
	podOK      = workspaceConditionSetterFactory(podFailureCondition, metav1.ConditionFalse)
)

type workspaceConditionSetter func(v1alpha1.Workspace, string, string) v1alpha1.Workspace

func workspaceConditionSetterFactory(condType string, status metav1.ConditionStatus) workspaceConditionSetter {
	return func(ws v1alpha1.Workspace, reason, message string) v1alpha1.Workspace {
		meta.SetStatusCondition(&ws.Status.Conditions, metav1.Condition{
			Type:    condType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
		return ws
	}
}
