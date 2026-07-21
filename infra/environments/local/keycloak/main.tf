module "realm" {
  source = "../../../modules/keycloak-realm"

  realm_name          = "local"
  admin_client_secret = var.admin_client_secret
}

module "org_local" {
  source = "../../../modules/keycloak-org"

  realm_id             = module.realm.id
  org_name             = "local"
  client_secret        = var.org_client_secret
  extra_default_scopes = [module.realm.groups_scope_name]
  redirect_uris        = ["http://localhost:3000/auth/keycloak"]
  web_origins          = ["http://localhost:3000"]
}

locals {
  dev_users = {
    admin   = { email = "admin@local.de", role = "Admin" }
    planner = { email = "planner@local.de", role = "Planner" }
    user    = { email = "user@local.de", role = "User" }
  }
}

resource "keycloak_user" "dev" {
  for_each = local.dev_users

  realm_id   = module.realm.id
  username   = each.key
  email      = each.value.email
  first_name = each.key
  last_name  = each.key
  enabled    = true

  initial_password {
    value     = each.key
    temporary = false
  }
}

resource "keycloak_user_roles" "dev" {
  for_each = local.dev_users

  realm_id = module.realm.id
  user_id  = keycloak_user.dev[each.key].id
  role_ids = [module.org_local.role_ids[each.value.role]]
}

resource "keycloak_user_groups" "dev" {
  for_each = local.dev_users

  realm_id  = module.realm.id
  user_id   = keycloak_user.dev[each.key].id
  group_ids = [module.org_local.group_id]
}
