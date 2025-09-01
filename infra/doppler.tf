data "aws_iam_policy_document" "doppler_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::299900769157:root"]
    }
    condition {
      test     = "StringEquals"
      variable = "sts:ExternalId"
      values   = [var.doppler_workspace_id]
    }
  }
}

resource "aws_iam_role" "doppler_role" {
  name               = "doppler-secrets-manager-role"
  assume_role_policy = data.aws_iam_policy_document.doppler_assume_role.json
}

data "aws_iam_policy_document" "doppler_secrets_manager_policy" {
  statement {
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:CreateSecret",
      "secretsmanager:DescribeSecret",
      "secretsmanager:PutSecretValue",
      "secretsmanager:UpdateSecret",
      "secretsmanager:DeleteSecret",
      "secretsmanager:TagResource"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_policy" "doppler_secrets_manager_policy" {
  name   = "DopplerSecretsManagerSync"
  policy = data.aws_iam_policy_document.doppler_secrets_manager_policy.json
}

resource "aws_iam_role_policy_attachment" "doppler_role_policy_attachment" {
  role       = aws_iam_role.doppler_role.name
  policy_arn = aws_iam_policy.doppler_secrets_manager_policy.arn
}
