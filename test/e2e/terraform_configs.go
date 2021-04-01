package e2e

import (
	"os/exec"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

// Terraform configs for e2e tests

var (
	// Terraform rootModuleConfig
	rootModuleConfig = `
terraform {
  required_providers {
	random = {
	  source  = "hashicorp/random"
	  version = "~> 3.0.0"
	}
  }
}

variable "suffix" {}

module "random" {
  source = "../modules/random"
  suffix = var.suffix
}

output "random_string" {
  value = "${module.random.random_string}-${var.namespace}-${var.workspace}"
}`

	randomModuleConfig = `
variable "suffix" {}

resource "random_id" "test" {
  byte_length = 2
}

output "random_string" {
  value = "${random_id.test.hex}-${var.suffix}"
}`
)

// Create terraform configs, git repo, and return path to root module
func createTerraformConfigs(t *testing.T) string {
	configs := testutil.NewTempDir(t)
	configs.Write("root/main.tf", []byte(rootModuleConfig))
	configs.Write("modules/random/main.tf", []byte(randomModuleConfig))

	// Create git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = configs.Root()
	require.NoError(t, cmd.Run())

	// Add remote
	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com/leg100/e2e.git")
	cmd.Dir = configs.Root()
	require.NoError(t, cmd.Run())

	path := configs.Path("root")
	// Log pwd for debugging broken e2e tests
	t.Logf("configuration path set to: %s", path)
	return path
}
