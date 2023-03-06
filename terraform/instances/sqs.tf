resource "aws_sqs_queue" "tagging_queue" {
  name       = "helm_tag_manager_queue.fifo"
  fifo_queue = true
  content_based_deduplication = true
}
