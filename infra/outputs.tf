output "lambda_function_url" {
  description = "The URL of the Lambda function."
  value       = aws_lambda_function_url.vo2_lambda_url.function_url
}
