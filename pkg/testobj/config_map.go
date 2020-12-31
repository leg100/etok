package testobj

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ConfigMap(namespace, name string, opts ...func(*corev1.ConfigMap)) *corev1.ConfigMap {
	var configMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, o := range opts {
		o(configMap)
	}

	return configMap
}
