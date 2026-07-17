resource "keycloak_openid_client" "this" {
  realm_id  = var.realm_id
  client_id = var.org_name

  access_type                  = "CONFIDENTIAL"
  service_accounts_enabled     = true
  direct_access_grants_enabled = true
  standard_flow_enabled        = false

  authorization {
    policy_enforcement_mode          = "ENFORCING"
    decision_strategy                = "AFFIRMATIVE"
    allow_remote_resource_management = true
    keep_defaults                    = false
  }
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
