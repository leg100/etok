terraform {
  backend "gcs" {
    bucket = "automatize-tfstate"
    prefix = "e2e"
  }
}

provider "random" {
  version = "~> 3.0.0"
}

variable "suffix" {}

module "random" {
  source = "../modules/random"
  suffix = var.suffix
}

output "random_string" {
  value = module.random.random_string
}
