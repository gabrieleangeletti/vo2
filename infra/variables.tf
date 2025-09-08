variable "aws_region" {
  description = "The AWS region to deploy resources in."
  type        = string
  default     = "eu-central-1"
}

variable "lambda_function_name" {
  description = "The name of the Lambda function."
  type        = string
  default     = "vo2-lambda"
}

variable "doppler_workspace_id" {
  description = "Your Doppler Workspace ID for the AWS integration."
  type        = string
  sensitive   = true
}

variable "doppler_secret_name" {
  description = "Your Doppler Secret Name for the AWS integration."
  type        = string
  sensitive   = true
}

variable "s3_bucket_name" {
  description = "The name of the S3 bucket."
  type        = string
}
