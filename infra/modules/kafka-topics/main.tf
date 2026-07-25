resource "kafka_topic" "this" {
  for_each = var.topics

  name               = each.key
  partitions         = each.value.partitions
  replication_factor = each.value.replication_factor
  config             = each.value.config
}
