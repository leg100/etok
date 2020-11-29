package runner

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	cacheVolumeName         = "cache"
	backendConfigVolumeName = "backendconfig"
	credentialsVolumeName   = "credentials"

	// SubPaths mounted from the PVC (they may well be used as the mount path
	// too)
	dotTerraformPath        = ".terraform/"
	localTerraformStatePath = "terraform.tfstate.d/"
	terraformBinPath        = "terraform-bins/"
)

// Runner represents a kubernetes pod on which a run's command is executed
// (typically terraform). It has two implementations: workspace and run.
type Runner interface {
	controllerutil.Object
	ContainerArgs() []string
	GetHandshake() bool
	GetHandshakeTimeout() string
	GetVerbosity() int
	WorkingDir() string
	PodName() string
}
