package backup

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func path(key client.ObjectKey) string {
	return fmt.Sprintf("%s.yaml", key)
}
