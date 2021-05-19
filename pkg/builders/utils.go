package builders

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SetNamespacedName(obj metav1.Object, key string) {
	parts := strings.Split(key, "/")
	if len(parts) > 1 {
		obj.SetNamespace(parts[0])
		obj.SetName(parts[1])
	} else {
		obj.SetName(parts[0])
	}
}
