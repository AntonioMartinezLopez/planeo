variable "kafka_bootstrap_admin_password" {
  description = "Bootstrap SCRAM admin password, must match dev/.env's KAFKA_BOOTSTRAP_ADMIN_PASSWORD"
  type        = string
  sensitive   = true
}
