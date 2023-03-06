resource "aws_ecr_repository" "helm_tag_manager" {
  name = "helm-tag-manager"
}

resource "aws_ecr_lifecycle_policy" "helm_tag_manager" {
  repository = aws_ecr_repository.helm_tag_manager.name


  policy = <<EOF
    {
        "rules": [
            {"rulePriority": 1,
            "description": "Keep last 10 images",
            "selection": {
                "tagStatus": "tagged",
                "tagPrefixList": ["v"],
                "countType": "imageCountMoreThan",
                "countNumber": 10
            },
            "action": {
                "type": "expire"
            }}
        ]
    }
    EOF
}


