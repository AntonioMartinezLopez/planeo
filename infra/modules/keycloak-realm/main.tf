resource "keycloak_realm" "this" {
  realm   = var.realm_name
  enabled = true

  access_token_lifespan    = var.access_token_lifespan
  sso_session_idle_timeout = var.sso_session_idle_timeout
  sso_session_max_lifespan = var.sso_session_max_lifespan
}

# services/core authenticates as this client (direct access grant, against the
# master realm) to obtain a token for calling the Keycloak Admin REST API
# against var.realm_name. Must live in "master", not var.realm_name - Admin
# API calls against any realm require a master-realm-authenticated token.
resource "keycloak_openid_client" "admin_dev" {
  realm_id  = "master"
  client_id = "admin-dev"

  access_type                  = "CONFIDENTIAL"
  client_secret                = var.admin_client_secret
  direct_access_grants_enabled = true
  service_accounts_enabled     = false
  standard_flow_enabled        = false
}
