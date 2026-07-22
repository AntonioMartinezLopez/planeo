provider "kafka" {
  bootstrap_servers = ["localhost:9094"]
  tls_enabled       = false
  sasl_mechanism    = "scram-sha256"
  sasl_username     = "admin"
  sasl_password     = var.kafka_bootstrap_admin_password
}
