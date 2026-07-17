output "id" {
  value = keycloak_realm.this.id
}

output "realm" {
  value = keycloak_realm.this.realm
}

output "groups_scope_id" {
  value = keycloak_openid_client_scope.groups.id
}

output "groups_scope_name" {
  value = keycloak_openid_client_scope.groups.name
}
