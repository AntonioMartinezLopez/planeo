variable "realm_id" {
  description = "ID of the realm the client belongs to"
  type        = string
}

variable "resource_server_id" {
  description = "resource_server_id of the client this authorization schema applies to"
  type        = string
}

variable "role_ids" {
  description = "map of role name => role ID, e.g. { Admin = \"<uuid>\", Planner = \"<uuid>\", User = \"<uuid>\" }"
  type        = map(string)
}

variable "permission_schema" {
  description = "resource name => scope name => list of role names granted that scope"
  type        = map(map(list(string)))
}
