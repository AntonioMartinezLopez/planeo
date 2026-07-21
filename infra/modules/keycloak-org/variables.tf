variable "realm_id" {
  description = "ID of the realm this organization belongs to"
  type        = string
}

variable "org_name" {
  description = "Name of the organization/tenant; used as the group name. Members authenticate against the realm's shared client (see keycloak-client) and are scoped to this organization via group membership."
  type        = string
}
