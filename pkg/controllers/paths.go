package controllers

const (
	// binMountPath is container path to terraform binaries
	binMountPath = "/terraform-bins"
	// binSubPath is path within persistent volume to mount on BinMountPath
	binSubPath = "terraform-bins/"

	// pluginMountPath is container path to terraform plugin cache
	pluginMountPath = "/plugin-cache"
	// pluginSubPath is path within persistent volume to mount on
	// pluginMountPath
	pluginSubPath = "plugin-cache/"

	// dotTerraformSubPath is path within persistent volume to mount on
	// <WorkingDir>/.terraform
	dotTerraformSubPath = ".terraform/"

	// workspaceDir is the directory in the container where the tarball is
	// extracted to
	workspaceDir = "/workspace"

	// variablesPath is the filename in <WorkingDir> containing declarations of
	// built-in variables such as namespace and workspace.
	variablesPath = "_etok_variables.tf"
)
