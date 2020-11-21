variable "suffix" {}

module "random" {
  source = "../modules/random"
  suffix = var.suffix
}

output "random_string" {
  value = module.random.random_string
}
