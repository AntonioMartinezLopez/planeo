output "resource_ids" {
  value = { for name, r in keycloak_openid_client_authorization_resource.resources : name => r.id }
}

output "permission_ids" {
  value = { for key, p in keycloak_openid_client_authorization_permission.permissions : key => p.id }
}
