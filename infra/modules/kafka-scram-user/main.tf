resource "random_password" "this" {
  length  = 32
  special = false
}

resource "kafka_user_scram_credential" "this" {
  username        = var.username
  scram_mechanism = "SCRAM-SHA-256"
  password        = random_password.this.result
}

resource "kafka_acl" "this" {
  for_each = { for i, a in var.acls : i => a }

  resource_name                = each.value.resource_name
  resource_type                = each.value.resource_type
  resource_pattern_type_filter = each.value.resource_pattern_type_filter
  acl_principal                = "User:${var.username}"
  acl_host                     = "*"
  acl_operation                = each.value.operation
  acl_permission_type          = each.value.permission_type
}
