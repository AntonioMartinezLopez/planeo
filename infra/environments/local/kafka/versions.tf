terraform {
  required_version = ">= 1.6.0"

  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "~> 0.13"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
