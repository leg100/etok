package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	runComplete   = runConditionSetterFactory(v1alpha1.RunCompleteCondition, metav1.ConditionTrue)
	runIncomplete = runConditionSetterFactory(v1alpha1.RunCompleteCondition, metav1.ConditionFalse)
	runFailed     = runConditionSetterFactory(v1alpha1.RunFailedCondition, metav1.ConditionTrue)
)

func workspaceFailure(message string) *metav1.Condition {
	return &metav1.Condition{
		Type:    v1alpha1.WorkspaceReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  v1alpha1.FailureReason,
		Message: message,
	}
}

func workspacePending(message string) *metav1.Condition {
	return &metav1.Condition{
		Type:    v1alpha1.WorkspaceReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  v1alpha1.PendingReason,
		Message: message,
	}
}

func workspaceUnknown(message string) *metav1.Condition {
	return &metav1.Condition{
		Type:    v1alpha1.WorkspaceReadyCondition,
		Status:  metav1.ConditionUnknown,
		Reason:  v1alpha1.UnknownReason,
		Message: message,
	}
}

type runConditionSetter func(*v1alpha1.Run, string, string)

func runConditionSetterFactory(condType string, status metav1.ConditionStatus) runConditionSetter {
	return func(run *v1alpha1.Run, reason, message string) {
		meta.SetStatusCondition(&run.Conditions, metav1.Condition{
			Type:    condType,
			Status:  status,
			Reason:  reason,
			Message: message,
		})
	}
}
