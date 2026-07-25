variable "topics" {
  description = "topic name => { partitions, replication_factor, config }"
  type = map(object({
    partitions         = number
    replication_factor = number
    config             = optional(map(string), {})
  }))
}
