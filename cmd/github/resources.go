package github

import (
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/pkg/labels"
)

var (
	// Chmod perms for github app private key
	readOnlyPerms int32 = 400

	// Labels used by service, deployment to identify pod
	selector = labels.MakeLabels(
		labels.App,
		labels.WebhookComponent,
	)
)

func deployment(namespace, image string, port int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "webhook",
					Containers: []corev1.Container{
						{
							Name:            "webhook",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"etok"},
							Args:            []string{"github", "run"},
							Env: []corev1.EnvVar{
								{
									Name:  "ETOK_PORT",
									Value: strconv.FormatInt(int64(port), 10),
								},
								{
									Name: "ETOK_APP_ID",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "creds",
											},
											Key: "id",
										},
									},
								},
								{
									Name: "ETOK_WEBHOOK_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "creds",
											},
											Key: "webhook-secret",
										},
									},
								},
								{
									Name:  "ETOK_KEY_PATH",
									Value: "/creds/key.pem",
								},
							},
							TerminationMessagePolicy: "FallbackToLogsOnError",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "creds",
									MountPath: "/creds",
								},
								{
									Name:      "repos",
									MountPath: "/repos",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "creds",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "creds",
									Items: []corev1.KeyToPath{
										{
											Key:  "key",
											Path: "key.pem",
											Mode: &readOnlyPerms,
										},
									},
								},
							},
						},
						{
							Name: "repos",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	return deployment
}

func isAvailable(c appsv1.DeploymentCondition) bool {
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

func service(namespace string, port int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook",
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     port,
					Protocol: "TCP",
				},
			},
			Selector: selector,
		},
	}
}

func serviceAccount(namespace string, annotations map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "webhook",
			Namespace:   namespace,
			Annotations: annotations,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
	}
}

func clusterRoleBinding(namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Namespace: namespace,
				Name:      "webhook",
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "webhook",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func clusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "webhook",
		},
		Rules: []rbacv1.PolicyRule{
			// Webhook needs to be able to create and monitor runs
			{
				Resources: []string{"runs"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
				APIGroups: []string{"etok.dev"},
			},
			// Webhook needs to create configmaps, in which to store terraform
			// configuration for a run
			{
				Resources: []string{"configmaps"},
				Verbs:     []string{"create"},
				APIGroups: []string{""},
			},
			// Webhook needs read-access to workspaces
			{
				Resources: []string{"workspaces"},
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"etok.dev"},
			},
		},
	}
}
