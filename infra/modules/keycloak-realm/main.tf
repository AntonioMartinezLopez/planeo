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

# Realm-scoped identity concern: emits a top-level "groups" claim (e.g.
# groups: ["/local"]) in every issued token, mirroring the retired
# auth/local/realm.json's custom client scope. Consumed by
# libs/middlewares/organization_validation.go via IsWithinOrganisation, which
# checks this claim for "/<organization>" membership.
resource "keycloak_openid_client_scope" "groups" {
  realm_id    = keycloak_realm.this.id
  name        = "groups"
  description = "Group membership"

  include_in_token_scope = true
}

resource "keycloak_openid_group_membership_protocol_mapper" "groups" {
  realm_id        = keycloak_realm.this.id
  client_scope_id = keycloak_openid_client_scope.groups.id
  name            = "group"
  claim_name      = "groups"
  full_path       = true

  add_to_id_token     = true
  add_to_access_token = true
  add_to_userinfo     = true
}
