resource "keycloak_group" "this" {
  realm_id = var.realm_id
  name     = var.org_name
}
