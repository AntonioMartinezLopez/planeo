output "email_received_producer_password" {
  value     = module.email_received_producer.password
  sensitive = true
}

output "core_email_received_consumer_password" {
  value     = module.core_email_received_consumer.password
  sensitive = true
}
