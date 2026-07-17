module "realm" {
  source = "../../../modules/keycloak-realm"

  realm_name          = "local"
  admin_client_secret = var.admin_client_secret
}
