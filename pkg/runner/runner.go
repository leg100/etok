package runner

import (
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	cacheVolumeName         = "cache"
	backendConfigVolumeName = "backendconfig"
	credentialsVolumeName   = "credentials"

	// Terraform 'data' directory. Mount path is relative to working directory.
	terraformDotPath = ".terraform/"

	// Directory in which local state is maintained. Mount path is relative to
	// working directory. Only used when a remote backend isn't configured.
	terraformLocalStatePath = "terraform.tfstate.d/"

	// Terraform binary path
	terraformBinMountPath = "/terraform-bins"
	terraformBinSubPath   = "terraform-bins/"
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
