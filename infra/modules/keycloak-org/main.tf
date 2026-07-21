resource "keycloak_openid_client" "this" {
  realm_id  = var.realm_id
  client_id = var.org_name

  access_type                  = "CONFIDENTIAL"
  client_secret                = var.client_secret
  service_accounts_enabled     = true
  direct_access_grants_enabled = true
  standard_flow_enabled        = true
  valid_redirect_uris          = var.redirect_uris
  web_origins                  = var.web_origins

  authorization {
    policy_enforcement_mode          = "ENFORCING"
    decision_strategy                = "AFFIRMATIVE"
    allow_remote_resource_management = true
    keep_defaults                    = false
  }
}

# Replaces the client's entire default scope list (Keycloak semantics), so
# the base list below must mirror the live stock defaults Keycloak assigns to
# a confidential client with service accounts enabled -- verified against
# GET /admin/realms/local/clients?clientId=local on the live dev realm.
resource "keycloak_openid_client_default_scopes" "this" {
  realm_id  = var.realm_id
  client_id = keycloak_openid_client.this.id

  default_scopes = concat(
    ["service_account", "web-origins", "acr", "profile", "roles", "basic", "email"],
    var.extra_default_scopes,
  )
}

resource "keycloak_role" "roles" {
  for_each  = toset(["Admin", "Planner", "User"])
  realm_id  = var.realm_id
  client_id = keycloak_openid_client.this.id
  name      = each.value
}

resource "keycloak_group" "this" {
  realm_id = var.realm_id
  name     = var.org_name
}

module "authz" {
  source = "../keycloak-client-authz"

  realm_id           = var.realm_id
  resource_server_id = keycloak_openid_client.this.resource_server_id
  role_ids           = { for name, role in keycloak_role.roles : name => role.id }
  permission_schema  = var.permission_schema
}
