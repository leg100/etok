package backup

import (
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrBucketNotFound = errors.New("backup bucket could not be found")
)

func path(key client.ObjectKey) string {
	return fmt.Sprintf("%s.yaml", key)
}
