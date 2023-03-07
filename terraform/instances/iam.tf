data aws_iam_policy_document helm_tag_manager {
  statement {
    effect = "Allow"
    actions = [
      "sqs:DeleteMessage",
      "sqs:GetQueueUrl",
      "sqs:ReceiveMessage",
      "sqs:SendMessage",
      "sqs:GetQueueAttributes"
    ]
    resources = [aws_sqs_queue.tagging_queue.arn]
  }
}

resource "aws_iam_user" "helm_tag_manager" {
  name = "helm-tag-manager"
}

resource "aws_iam_policy" "helm_tag_manager" {
  name        = "helm-tag-manager"
  path        = "/"
  description = "Helm Tag Manager"
  policy      = data.aws_iam_policy_document.helm_tag_manager.json
}

resource "aws_iam_user_policy_attachment" "helm_tag_manager" {
  user       = aws_iam_user.helm_tag_manager.name
  policy_arn = aws_iam_policy.helm_tag_manager.arn
}

resource "aws_iam_access_key" "helm_tag_manager" {
  user = aws_iam_user.helm_tag_manager.name
}
