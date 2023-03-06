data "aws_iam_policy_document" "auth_lambda" {
  statement {
    effect = "Allow"
    actions = [
      "lambda:InvokeFunction",
    ]
    resources = [aws_lambda_function.helm_tag_manager_auth.arn]
  }
}

resource "aws_iam_role" "auth_lambda_role" {
  name               = "helm_tag_manager_auth"
  assume_role_policy = <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
     "Action": "sts:AssumeRole",
     "Principal": {
       "Service": [
            "apigateway.amazonaws.com"
        ]
     },
     "Effect": "Allow",
     "Sid": ""
   }
 ]
}
EOF
}

resource "aws_iam_policy" "auth_lambda" {
  name        = "aws_iam_policy_for_helm_tag_manager_auth"
  path        = "/"
  description = "AWS IAM Policy for managing aws lambda role"
  policy      = data.aws_iam_policy_document.auth_lambda.json
}

resource "aws_iam_role_policy_attachment" "auth_lambda_api_attach" {
  role       = aws_iam_role.auth_lambda_role.name
  policy_arn = aws_iam_policy.auth_lambda.arn
}

resource "aws_apigatewayv2_api" "helm_tag_manager" {
  name          = "helm-tag-manager-http-api"
  protocol_type = "HTTP"
}

resource "aws_apigatewayv2_route" "helm_tag_manager" {
  api_id             = aws_apigatewayv2_api.helm_tag_manager.id
  authorizer_id      = aws_apigatewayv2_authorizer.helm_tag_manager.id
  authorization_type = "CUSTOM"
  route_key          = "POST /"
  target             = "integrations/${aws_apigatewayv2_integration.helm_tag_manager.id}"
}

resource "aws_apigatewayv2_integration" "helm_tag_manager" {
  api_id              = aws_apigatewayv2_api.helm_tag_manager.id
  credentials_arn     = aws_iam_role.lambda_role.arn
  description         = "Helm SQS Integration"
  integration_type    = "AWS_PROXY"
  integration_subtype = "SQS-SendMessage"
  connection_type     = "INTERNET"

  request_parameters = {
    "QueueUrl"               = aws_sqs_queue.tagging_queue.url
    "MessageBody"            = "$request.body.message",
    "MessageDeduplicationId" = "test"
    "MessageGroupId"         = "test"
  }
}

resource "aws_apigatewayv2_stage" "helm_tag_manager" {
  api_id      = aws_apigatewayv2_api.helm_tag_manager.id
  name        = "helm_tag_manager"
  auto_deploy = true


  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gateway_logging.arn
    format          = "$context.identity.sourceIp - - [$context.requestTime] \"$context.httpMethod $context.routeKey $context.protocol\" $context.status $context.responseLength $context.requestId $context.authorizer.error $context.error.message"
  }

  default_route_settings {
    logging_level            = "INFO"
    data_trace_enabled       = true
    detailed_metrics_enabled = true
    throttling_burst_limit   = 50
    throttling_rate_limit    = 200
  }
}

resource "aws_apigatewayv2_authorizer" "helm_tag_manager" {
  api_id                            = aws_apigatewayv2_api.helm_tag_manager.id
  name                              = "helm_tag_manager_auth"
  authorizer_type                   = "REQUEST"
  authorizer_uri                    = aws_lambda_function.helm_tag_manager_auth.invoke_arn
  authorizer_credentials_arn        = aws_iam_role.auth_lambda_role.arn
  authorizer_payload_format_version = "2.0"
  enable_simple_responses           = true
  identity_sources = [
    "$request.header.Authorization"
  ]
}


resource "aws_cloudwatch_log_group" "api_gateway_logging" {
  name              = "helm-tag-manager-api/${aws_apigatewayv2_api.helm_tag_manager.id}"
  retention_in_days = 30
  skip_destroy      = true
}
