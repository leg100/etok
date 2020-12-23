package e2e

import (
	"os"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/stretchr/testify/require"
)

// Terraform configs for e2e tests

var (
	// Terraform rootModuleConfig
	rootModuleConfig = `
terraform {
  backend "gcs" {
	bucket = "automatize-tfstate"
	prefix = "e2e"
  }
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
  value = module.random.random_string
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

// Create terraform configs, and set present working directory to root module
// path
func createTerraformConfigs(t *testing.T) {
	configs := testutil.NewTempDir(t)
	configs.Write("root/main.tf", []byte(rootModuleConfig))
	configs.Write("modules/random/main.tf", []byte(randomModuleConfig))

	testutil.Chdir(t, configs.Path("root"))

	// Log pwd for debugging broken e2e tests
	pwd, err := os.Getwd()
	require.NoError(t, err)
	t.Logf("%s: working directory set to: %s", t.Name(), pwd)
}
