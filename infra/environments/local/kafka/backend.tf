terraform {
  backend "local" {
    path = "../../../.tofu-state/local-kafka.tfstate"
  }
}
