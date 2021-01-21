package controllers

import (
	v1alpha1 "github.com/leg100/etok/api/etok.dev/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func runFailed(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:    v1alpha1.RunFailedCondition,
		Status:  metav1.ConditionTrue,
		Reason:  reason,
		Message: message,
	}
}

func runIncomplete(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:    v1alpha1.RunCompleteCondition,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	}
}

func runComplete(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:    v1alpha1.RunCompleteCondition,
		Status:  metav1.ConditionTrue,
		Reason:  reason,
		Message: message,
	}
}
