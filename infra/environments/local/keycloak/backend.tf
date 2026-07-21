terraform {
  backend "local" {
    path = "../../../.tofu-state/local-keycloak.tfstate"
  }
}
