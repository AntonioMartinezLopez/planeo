variable "kc_base_url" {
  description = "Base URL of the local Keycloak instance"
  type        = string
}

variable "kc_admin_username" {
  description = "Master-realm superuser username (KEYCLOAK_ADMIN in docker-compose)"
  type        = string
}

variable "kc_admin_password" {
  description = "Master-realm superuser password (KEYCLOAK_ADMIN_PASSWORD in docker-compose)"
  type        = string
  sensitive   = true
}

variable "admin_client_secret" {
  description = "Fixed secret for the admin-dev client, must match services/core/.env's KC_ADMIN_CLIENT_SECRET"
  type        = string
  sensitive   = true
}
