data "aws_iam_policy_document" "lambda" {
  statement {
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    resources = ["arn:aws:logs:*:*:*"]
  }
  statement {
    effect = "Allow"
    actions = [
      "sqs:SendMessage",
    ]
    resources = [aws_sqs_queue.tagging_queue.arn]
  }
}

resource "aws_iam_role" "lambda_role" {
  name               = "helm_tag_manager_publisher"
  assume_role_policy = <<EOF
{
 "Version": "2012-10-17",
 "Statement": [
   {
     "Action": "sts:AssumeRole",
     "Principal": {
       "Service": [
            "apigateway.amazonaws.com",
            "lambda.amazonaws.com"
        ]
     },
     "Effect": "Allow",
     "Sid": ""
   }
 ]
}
EOF
}

resource "aws_iam_policy" "iam_policy_for_lambda" {
  name        = "aws_iam_policy_for_terraform_aws_lambda_role"
  path        = "/"
  description = "AWS IAM Policy for managing aws lambda role"
  policy      = data.aws_iam_policy_document.lambda.json
}

resource "aws_iam_role_policy_attachment" "attach_iam_policy_to_iam_role" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.iam_policy_for_lambda.arn
}

data "archive_file" "auth_zip" {
  type        = "zip"
  source_dir  = "${path.module}/../../lambda/auth"
  output_path = "${path.module}/../../lambda/auth/auth.zip"

}

resource "aws_lambda_function" "helm_tag_manager_auth" {
  filename         = "${path.module}/../../lambda/auth/auth.zip"
  function_name    = "helm_tag_manager_auth"
  role             = aws_iam_role.lambda_role.arn
  handler          = "index.lambda_handler"
  runtime          = "python3.8"
  depends_on       = [aws_iam_role_policy_attachment.attach_iam_policy_to_iam_role]
  source_code_hash = data.archive_file.auth_zip.output_base64sha256

  environment {
    variables = {
      API_KEY = aws_ssm_parameter.api_key.value
    }
  }
}
