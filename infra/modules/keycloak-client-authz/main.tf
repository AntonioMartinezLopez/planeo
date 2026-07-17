locals {
  # Flatten the nested permission_schema map into one entry per resource:scope
  # pair, e.g. "Task:read" => { resource = "Task", scope = "read", roles = [...] }
  resource_scope_pairs = flatten([
    for resource, scopes in var.permission_schema : [
      for scope, roles in scopes : {
        key      = "${resource}:${scope}"
        resource = resource
        scope    = scope
        roles    = roles
      }
    ]
  ])
  resource_scope_map = { for p in local.resource_scope_pairs : p.key => p }

  resource_names = distinct([for p in local.resource_scope_pairs : p.resource])
  scope_names    = distinct([for p in local.resource_scope_pairs : p.scope])

  # scope names used by each resource, needed for that resource's own `scopes` set
  resource_scopes = {
    for resource in local.resource_names : resource => distinct([
      for p in local.resource_scope_pairs : p.scope if p.resource == resource
    ])
  }
}

resource "keycloak_openid_client_authorization_scope" "scopes" {
  for_each = toset(local.scope_names)

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = each.value
}

resource "keycloak_openid_client_authorization_resource" "resources" {
  for_each = toset(local.resource_names)

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = each.value
  scopes             = local.resource_scopes[each.value]
}

resource "keycloak_openid_client_role_policy" "role_policies" {
  for_each = var.role_ids

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = "${each.key}-role-policy"
  type               = "role"
  logic              = "POSITIVE"
  decision_strategy  = "UNANIMOUS"

  role {
    id       = each.value
    required = true
  }
}

resource "keycloak_openid_client_authorization_permission" "permissions" {
  for_each = local.resource_scope_map

  realm_id           = var.realm_id
  resource_server_id = var.resource_server_id
  name               = each.key
  type               = "scope"
  decision_strategy  = "AFFIRMATIVE"

  resources = [keycloak_openid_client_authorization_resource.resources[each.value.resource].id]
  scopes    = [keycloak_openid_client_authorization_scope.scopes[each.value.scope].id]
  policies  = [for role in each.value.roles : keycloak_openid_client_role_policy.role_policies[role].id]
}
