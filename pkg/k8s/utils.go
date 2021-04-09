package k8s

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ContainerStatusByName returns the ContainerStatus object for a container with
// a given name on the pod. Includes init containers.
func ContainerStatusByName(pod *corev1.Pod, name string) *corev1.ContainerStatus {
	allContainerStatuses := append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...)
	for _, status := range allContainerStatuses {
		if status.Name == name {
			return &status
		}
	}
	return nil
}

// DeploymentIsReady will poll the kubernetes API server to see if the
// deployment is ready to service user requests.
func DeploymentIsReady(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout, interval time.Duration) error {
	var readyObservations int32
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range deployment.Status.Conditions {
			if deploymentIsAvailable(cond) {
				readyObservations++
			}
		}
		// Make sure we query the deployment enough times to see the state change, provided there is one.
		if readyObservations > 4 {
			return true, nil
		} else {
			return false, nil
		}
	})
}

func deploymentIsAvailable(c appsv1.DeploymentCondition) bool {
	// Make sure that the deployment has been available for at least 10 seconds.
	// This is because the deployment can show as Ready momentarily before the pods fall into a CrashLoopBackOff.
	// See podutils.IsPodAvailable upstream for similar logic with pods
	if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
		if !c.LastTransitionTime.IsZero() && c.LastTransitionTime.Add(10*time.Second).Before(time.Now()) {
			return true
		}
	}
	return false
}
