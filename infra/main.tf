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
      HISTORICAL_DATA_QUEUE_URL = aws_sqs_queue.historical_data_queue.url
    }
  }
}

resource "aws_lambda_function_url" "vo2_lambda_url" {
  function_name      = aws_lambda_function.vo2_lambda.function_name
  authorization_type = "NONE"
}

# SQS Queue for historical data pulling jobs
resource "aws_sqs_queue" "historical_data_queue" {
  name                       = "vo2-historical-data-queue"
  visibility_timeout_seconds = 300     # 5 minutes
  message_retention_seconds  = 1209600 # 14 days
  receive_wait_time_seconds  = 20      # long polling

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.historical_data_dlq.arn
    maxReceiveCount     = 3
  })
}

# Dead Letter Queue for failed historical data jobs
resource "aws_sqs_queue" "historical_data_dlq" {
  name                      = "vo2-historical-data-dlq"
  message_retention_seconds = 1209600 # 14 days
}

# Lambda trigger from SQS
resource "aws_lambda_event_source_mapping" "historical_data_queue_trigger" {
  event_source_arn = aws_sqs_queue.historical_data_queue.arn
  function_name    = aws_lambda_function.vo2_lambda.function_name
  batch_size       = 3
}

