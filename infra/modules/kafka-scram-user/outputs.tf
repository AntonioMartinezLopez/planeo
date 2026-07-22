output "username" {
  value = kafka_user_scram_credential.this.username
}

output "password" {
  value     = random_password.this.result
  sensitive = true
}
