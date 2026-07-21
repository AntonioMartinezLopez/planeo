variable "realm_name" {
  description = "Name of the realm to create (also becomes its ID)"
  type        = string
}

variable "access_token_lifespan" {
  description = "How long an access token stays valid (Go duration string)"
  type        = string
  default     = "10h"
}

variable "sso_session_idle_timeout" {
  description = "How long an SSO session stays valid without activity (Go duration string)"
  type        = string
  default     = "720h"
}

variable "sso_session_max_lifespan" {
  description = "Maximum lifetime of an SSO session (Go duration string)"
  type        = string
  default     = "240h"
}

variable "admin_client_secret" {
  description = "Fixed secret for the master-realm admin-dev client that services/core uses to call the Keycloak Admin API"
  type        = string
  sensitive   = true
}
