provider "google" {
  project     = "master-anagram-224816"
  region      = "europe-west2"
}

terraform {
  backend "gcs" {
    bucket  = "master-anagram-224816-tfstate"
  }
}
