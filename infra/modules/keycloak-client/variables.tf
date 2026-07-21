variable "realm_id" {
  description = "ID of the realm this client belongs to"
  type        = string
}

variable "client_name" {
  description = "client_id of the shared OIDC client for this realm/environment; every organization (Keycloak group) in the realm authenticates against this one client"
  type        = string
}

variable "client_secret" {
  description = "Fixed secret for this client"
  type        = string
  sensitive   = true
}

variable "extra_default_scopes" {
  description = "Additional realm client scope names to add to this client's default scopes, beyond Keycloak's stock defaults (service_account, web-origins, acr, profile, roles, basic, email)"
  type        = list(string)
  default     = []
}

variable "redirect_uris" {
  description = "Valid redirect URIs for the browser-based (standard flow) OIDC login, e.g. the web app's OAuth callback route"
  type        = list(string)
  default     = []
}

variable "web_origins" {
  description = "Allowed CORS web origins for browser-based clients using this client"
  type        = list(string)
  default     = []
}

variable "permission_schema" {
  description = "resource name => scope name => list of role names granted that scope. Defaults to the schema verified against auth/local/client_rbac.json."
  type        = map(map(list(string)))
  default = {
    Task = {
      read   = ["Planner", "User"]
      create = ["Planner"]
      update = ["Planner", "User"]
      delete = ["Planner"]
    }
    Announcement = {
      read   = ["Admin"]
      create = ["Admin"]
      update = ["Admin"]
      delete = ["Admin"]
    }
    Group = {
      read   = ["Planner", "User"]
      create = ["Planner"]
      update = ["Planner", "User"]
      delete = ["Planner"]
    }
    Conversation = {
      read   = ["Planner", "User"]
      create = ["Planner", "User"]
      update = ["Planner", "User"]
      delete = ["Planner", "User"]
    }
    Reminder = {
      read   = ["Planner", "User"]
      create = ["Planner"]
      update = ["Planner"]
      delete = ["Planner"]
    }
    Organization = {
      manage = ["Admin"]
    }
    User = {
      read   = ["Admin"]
      create = ["Admin"]
      update = ["Admin"]
      delete = ["Admin"]
    }
    Role = {
      read = ["Admin"]
    }
    Userinfo = {
      read = ["Planner", "Admin", "User"]
    }
    Request = {
      read   = ["Planner", "Admin"]
      create = ["Planner", "Admin"]
      update = ["Planner", "Admin"]
      delete = ["Planner", "Admin"]
    }
    Category = {
      read   = ["Planner", "Admin", "User"]
      create = ["Admin", "Planner"]
      update = ["Admin", "Planner"]
      delete = ["Admin"]
    }
  }
}
