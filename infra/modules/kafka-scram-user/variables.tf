variable "username" {
  description = "SCRAM username for this Kafka client service"
  type        = string
}

variable "acls" {
  description = "ACL grants for this user, e.g. [{ resource_type = \"Topic\", resource_name = \"email-received\", resource_pattern_type_filter = \"Literal\", operation = \"Write\", permission_type = \"Allow\" }]"
  type = list(object({
    resource_type                = string
    resource_name                = string
    resource_pattern_type_filter = string
    operation                    = string
    permission_type              = string
  }))
}
