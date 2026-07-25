module "topics" {
  source = "../../../modules/kafka-topics"

  topics = {
    "email-received" = {
      partitions         = 3
      replication_factor = 1
      config = {
        "retention.ms" = "604800000"
      }
    }
  }
}

module "email_received_producer" {
  source = "../../../modules/kafka-scram-user"

  username = "email-received-producer"
  acls = [
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Write", permission_type = "Allow" },
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Describe", permission_type = "Allow" },
  ]

  depends_on = [module.topics]
}

module "core_email_received_consumer" {
  source = "../../../modules/kafka-scram-user"

  username = "core-email-received-consumer"
  acls = [
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Read", permission_type = "Allow" },
    { resource_type = "Topic", resource_name = "email-received", resource_pattern_type_filter = "Literal", operation = "Describe", permission_type = "Allow" },
    { resource_type = "Group", resource_name = "core-email-received-consumer", resource_pattern_type_filter = "Literal", operation = "Read", permission_type = "Allow" },
    { resource_type = "Group", resource_name = "core-email-received-consumer", resource_pattern_type_filter = "Literal", operation = "Describe", permission_type = "Allow" },
  ]

  depends_on = [module.topics]
}
