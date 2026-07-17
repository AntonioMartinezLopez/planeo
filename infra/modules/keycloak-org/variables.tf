variable "realm_id" {
  description = "ID of the realm this organization belongs to"
  type        = string
}

variable "org_name" {
  description = "Name of the organization/tenant; used as the client_id and group name"
  type        = string
}

variable "client_secret" {
  description = "Fixed secret for this organization's OIDC client"
  type        = string
  sensitive   = true
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
