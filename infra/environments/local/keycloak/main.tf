module "realm" {
  source = "../../../modules/keycloak-realm"

  realm_name          = "local"
  admin_client_secret = var.admin_client_secret
}

module "org_local" {
  source = "../../../modules/keycloak-org"

  realm_id = module.realm.id
  org_name = "local"
}
