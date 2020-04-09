provider "google" {
  project     = "master-anagram-224816"
  region      = "europe-west2"
}

terraform {
  backend "gcs" {
    bucket  = "master-anagram-224816-tfstate"
  }
}

resource "random_id" "test" {
  byte_length = 2
}

resource "google_storage_bucket" "test" {
  name     = "master-anagram-224816-${random_id.test.hex}"
  location = "EU"
}
