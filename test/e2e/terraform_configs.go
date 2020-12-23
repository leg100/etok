package e2e

import (
	"testing"

	"github.com/leg100/etok/pkg/testutil"
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

// Create terraform configs, and return path to root module
func createTerraformConfigs(t *testing.T) string {
	configs := testutil.NewTempDir(t)
	configs.Write("root/main.tf", []byte(rootModuleConfig))
	configs.Write("modules/random/main.tf", []byte(randomModuleConfig))

	path := configs.Path("root")
	// Log pwd for debugging broken e2e tests
	t.Logf("configuration path set to: %s", path)
	return path
}
