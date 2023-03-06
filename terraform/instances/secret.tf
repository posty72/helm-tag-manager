resource "random_password" "api_key" {
  length  = 40
  special = false
  numeric = true
  upper   = true
  lower   = true
}

resource "aws_ssm_parameter" "api_key" {
  name  = "/helm_tag_manager/api_key"
  type  = "SecureString"
  value = random_password.api_key.result
}
