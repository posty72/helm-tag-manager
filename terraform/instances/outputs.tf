output lamda_url {
  value = "${aws_apigatewayv2_api.helm_tag_manager.api_endpoint}/helm_tag_manager/"
}

output token {
  value     = aws_ssm_parameter.api_key.value
  sensitive = true
}

output sqs_url {
  value = aws_sqs_queue.tagging_queue.url
}

output ecr_url {
  value = aws_ecr_repository.helm_tag_manager.repository_url
}
