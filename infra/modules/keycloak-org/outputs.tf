output "client_id" {
  value = keycloak_openid_client.this.id
}

output "resource_server_id" {
  value = keycloak_openid_client.this.resource_server_id
}

output "group_id" {
  value = keycloak_group.this.id
}

output "role_ids" {
  value = { for name, role in keycloak_role.roles : name => role.id }
}
