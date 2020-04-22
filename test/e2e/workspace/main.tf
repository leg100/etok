terraform {
  backend "gcs" {
    bucket  = "master-anagram-224816-tfstate"
  }
}

variable "suffix" {}

resource "random_id" "test" {
  byte_length = 2
}

output "random_string" {
  value = "${random_id.test.hex}-${var.suffix}"
}
