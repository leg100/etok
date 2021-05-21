package github

import (
	"path/filepath"

	"github.com/leg100/etok/pkg/controllers"
)

// Path to a terraform plan file
type planPath string

// Construct a plan path using a global plans dir
func newPlanPath(name string) planPath {
	return planPath(filepath.Join(controllers.PlansMountPath, name))
}
