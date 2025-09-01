resource "aws_lambda_function" "vo2_lambda" {
  function_name = var.lambda_function_name
  role          = aws_iam_role.lambda_exec.arn

  package_type  = "Zip"
  filename      = "../bin/lambda.zip"
  handler       = "main"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  timeout       = 30

  source_code_hash = filebase64sha256("../bin/lambda.zip")

  layers = [
    "arn:aws:lambda:${var.aws_region}:187925254637:layer:AWS-Parameters-and-Secrets-Lambda-Extension-Arm64:18"
  ]

  environment {
    variables = {
      SECRETS_EXTENSION_ENABLED = "true"
      DOPPLER_SECRET_NAME       = var.doppler_secret_name
    }
  }
}

resource "aws_lambda_function_url" "vo2_lambda_url" {
  function_name      = aws_lambda_function.vo2_lambda.function_name
  authorization_type = "NONE"
}

