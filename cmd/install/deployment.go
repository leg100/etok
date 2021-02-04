package install

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/leg100/etok/pkg/backup"
	"github.com/leg100/etok/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/pkg/labels"
)

type podTemplateOption func(*podTemplateConfig)

type podTemplateConfig struct {
	image       string
	envVars     []corev1.EnvVar
	annotations map[string]string
	withSecret  bool
}

func WithImage(image string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.image = image
	}
}

func WithAnnotations(annotations map[string]string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.annotations = annotations
	}
}

func WithSecret(secretPresent bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.withSecret = secretPresent

	}
}

func WithBackupConfig(cfg *backup.Config) podTemplateOption {
	return func(c *podTemplateConfig) {
		for _, ev := range cfg.GetEnvVars() {
			c.envVars = append(c.envVars, ev)
		}
	}
}

func deployment(namespace string, opts ...podTemplateOption) *appsv1.Deployment {
	c := &podTemplateConfig{
		image: version.Image,
	}

	for _, opt := range opts {
		opt(c)
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etok",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "etok",
					Containers: []corev1.Container{
						{
							Name:            "operator",
							Image:           c.image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"etok"},
							Args:            []string{"operator"},
							Env: []corev1.EnvVar{
								{
									Name:  "WATCH_NAMESPACE",
									Value: "",
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "OPERATOR_NAME",
									Value: "etok",
								},
								{
									Name:  "ETOK_IMAGE",
									Value: c.image,
								},
							},
							TerminationMessagePolicy: "FallbackToLogsOnError",
						},
					},
				},
			},
		},
	}

	// Add environment variables to container
	for _, ev := range c.envVars {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, ev)
	}

	// Label selector for operator pod.  It must match the pod template's
	// labels.
	selector := labels.MakeLabels(
		labels.App,
		labels.OperatorComponent,
	)
	deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
	deployment.Spec.Template.Labels = selector

	if c.withSecret {
		deployment.Spec.Template.Spec.Containers[0].EnvFrom = append(deployment.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "etok",
				},
			},
		})
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
