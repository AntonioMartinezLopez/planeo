output "topic_names" {
  value = [for t in kafka_topic.this : t.name]
}
