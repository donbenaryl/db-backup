terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "aws" {
  profile = "don"
  region = var.aws_region
}

# Data source for current AWS account
data "aws_caller_identity" "current" {}

# Data source for current AWS region
data "aws_region" "current" {}

# Local values for database environment variables
locals {
  database_env_vars = {
    for i, db in var.databases : "DB_${i}_HOST" => db.host
  }
  database_port_vars = {
    for i, db in var.databases : "DB_${i}_PORT" => tostring(db.port)
  }
  database_username_vars = {
    for i, db in var.databases : "DB_${i}_USERNAME" => db.username
  }
  database_password_vars = {
    for i, db in var.databases : "DB_${i}_PASSWORD" => db.password
  }
  database_name_vars = {
    for i, db in var.databases : "DB_${i}_DATABASE" => db.database
  }
  database_ssl_vars = {
    for i, db in var.databases : "DB_${i}_SSL_MODE" => db.ssl_mode
  }
  
  # Combine all database environment variables
  all_database_env_vars = merge(
    local.database_env_vars,
    local.database_port_vars,
    local.database_username_vars,
    local.database_password_vars,
    local.database_name_vars,
    local.database_ssl_vars
  )
}

# S3 bucket for storing backup files
# This bucket is configured to never be deleted even on terraform destroy
resource "aws_s3_bucket" "backup_bucket" {
  bucket = var.backup_bucket_name

  # Prevent accidental deletion
  lifecycle {
    prevent_destroy = true
  }

  tags = {
    Name        = "PostgreSQL Backup Storage"
    Environment = var.environment
    Project     = "db-backuper"
    Purpose     = "Database backups"
  }
}

# S3 bucket for Lambda deployment packages
resource "aws_s3_bucket" "lambda_deployments" {
  bucket = "${var.function_name}-deployments-${random_string.bucket_suffix.result}"

  tags = {
    Name        = "Lambda Deployment Packages"
    Environment = var.environment
    Project     = "db-backuper"
    Purpose     = "Lambda deployments"
  }
}

# Random string for unique bucket names
resource "random_string" "bucket_suffix" {
  length  = 8
  special = false
  upper   = false
}

# S3 bucket versioning
resource "aws_s3_bucket_versioning" "backup_bucket_versioning" {
  bucket = aws_s3_bucket.backup_bucket.id
  versioning_configuration {
    status = "Enabled"
  }
}

# S3 bucket versioning for Lambda deployments
resource "aws_s3_bucket_versioning" "lambda_deployments_versioning" {
  bucket = aws_s3_bucket.lambda_deployments.id
  versioning_configuration {
    status = "Enabled"
  }
}

# S3 bucket server-side encryption
resource "aws_s3_bucket_server_side_encryption_configuration" "backup_bucket_encryption" {
  bucket = aws_s3_bucket.backup_bucket.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# S3 bucket server-side encryption for Lambda deployments
resource "aws_s3_bucket_server_side_encryption_configuration" "lambda_deployments_encryption" {
  bucket = aws_s3_bucket.lambda_deployments.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# S3 bucket lifecycle configuration for backup retention
resource "aws_s3_bucket_lifecycle_configuration" "backup_bucket_lifecycle" {
  bucket = aws_s3_bucket.backup_bucket.id

  rule {
    id     = "backup_retention"
    status = "Enabled"

    filter {
      prefix = ""
    }

    expiration {
      days = var.backup_retention_days
    }

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

# S3 bucket public access block
resource "aws_s3_bucket_public_access_block" "backup_bucket_pab" {
  bucket = aws_s3_bucket.backup_bucket.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# S3 bucket public access block for Lambda deployments
resource "aws_s3_bucket_public_access_block" "lambda_deployments_pab" {
  bucket = aws_s3_bucket.lambda_deployments.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# IAM role for Lambda function
resource "aws_iam_role" "lambda_role" {
  name = "${var.function_name}-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name        = "${var.function_name}-role"
    Environment = var.environment
    Project     = "db-backuper"
  }
}

# IAM policy for Lambda to access S3
resource "aws_iam_policy" "lambda_s3_policy" {
  name        = "${var.function_name}-s3-policy"
  description = "Policy for Lambda to access S3 buckets"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          aws_s3_bucket.backup_bucket.arn,
          "${aws_s3_bucket.backup_bucket.arn}/*",
          aws_s3_bucket.lambda_deployments.arn,
          "${aws_s3_bucket.lambda_deployments.arn}/*"
        ]
      }
    ]
  })
}

# IAM policy for Lambda basic execution
resource "aws_iam_policy" "lambda_basic_policy" {
  name        = "${var.function_name}-basic-policy"
  description = "Basic execution policy for Lambda"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:*"
      }
    ]
  })
}

# Attach policies to Lambda role
resource "aws_iam_role_policy_attachment" "lambda_s3_attachment" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_s3_policy.arn
}

resource "aws_iam_role_policy_attachment" "lambda_basic_attachment" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_basic_policy.arn
}

# Build Lambda deployment package using Docker
resource "null_resource" "lambda_build" {
  provisioner "local-exec" {
    command = <<-EOT
      cd ${path.module}/..
      docker build -f Dockerfile.lambda -t db-backuper-lambda .
      docker create --name temp-container db-backuper-lambda
      docker cp temp-container:/bootstrap ./cmd/lambda/bootstrap
      docker rm temp-container
    EOT
  }

  triggers = {
    source_hash = filemd5("${path.module}/../cmd/lambda/main.go")
    dockerfile_hash = filemd5("${path.module}/../Dockerfile.lambda")
  }
}

# Create deployment package - only include the built binary
data "archive_file" "lambda_zip" {
  type        = "zip"
  source_dir  = "../cmd/lambda"
  output_path = "/tmp/db-backuper-lambda.zip"
  excludes = [
    "*.go"  # Exclude source files, only include the built binary
  ]

  depends_on = [null_resource.lambda_build]
}

# Upload Lambda deployment package to S3
resource "aws_s3_object" "lambda_deployment_package" {
  bucket = aws_s3_bucket.lambda_deployments.id
  key    = "lambda-deployment-package.zip"
  source = data.archive_file.lambda_zip.output_path
  etag   = data.archive_file.lambda_zip.output_md5
}

# Lambda function for PostgreSQL backup operations
resource "aws_lambda_function" "backup_function" {
  s3_bucket        = aws_s3_bucket.lambda_deployments.id
  s3_key          = aws_s3_object.lambda_deployment_package.key
  function_name    = var.function_name
  role            = aws_iam_role.lambda_role.arn
  handler         = "bootstrap"
  source_code_hash = data.archive_file.lambda_zip.output_base64sha256
  runtime         = "provided.al2"
  timeout         = 900 # 15 minutes
  memory_size     = 512
  description     = "PostgreSQL database backup service - runs every 3 hours"

  environment {
    variables = merge(
      {
        AWS_BUCKET              = aws_s3_bucket.backup_bucket.bucket
        BACKUP_RETENTION_DAYS   = var.backup_retention_days
        BACKUP_SCHEDULE         = "0 */3 * * *" # Every 3 hours
        BACKUP_PREFIX           = var.backup_prefix
        LOG_LEVEL               = var.log_level
        LOG_FORMAT              = "json"
      },
      local.all_database_env_vars
    )
  }

  depends_on = [
    aws_iam_role_policy_attachment.lambda_s3_attachment,
    aws_iam_role_policy_attachment.lambda_basic_attachment,
    aws_cloudwatch_log_group.lambda_logs
  ]

  tags = {
    Name        = var.function_name
    Environment = var.environment
    Project     = "db-backuper"
  }
}

# CloudWatch Log Group for Lambda
resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = var.log_retention_days

  tags = {
    Name        = "${var.function_name}-logs"
    Environment = var.environment
    Project     = "db-backuper"
  }
}

# EventBridge rule to trigger Lambda every 3 hours
resource "aws_cloudwatch_event_rule" "backup_schedule" {
  name                = "${var.function_name}-schedule"
  description         = "Trigger backup function every 3 hours"
  schedule_expression = "rate(3 hours)"

  tags = {
    Name        = "${var.function_name}-schedule"
    Environment = var.environment
    Project     = "db-backuper"
  }
}

# EventBridge target to invoke Lambda
resource "aws_cloudwatch_event_target" "lambda_target" {
  rule      = aws_cloudwatch_event_rule.backup_schedule.name
  target_id = "BackupLambdaTarget"
  arn       = aws_lambda_function.backup_function.arn
}

# Permission for EventBridge to invoke Lambda
resource "aws_lambda_permission" "allow_eventbridge" {
  statement_id  = "AllowExecutionFromEventBridge"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.backup_function.function_name
  principal     = "events.amazonaws.com"
  source_arn    = aws_cloudwatch_event_rule.backup_schedule.arn
}

# CloudWatch alarm for Lambda errors
resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  alarm_name          = "${var.function_name}-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Sum"
  threshold           = "0"
  alarm_description   = "This metric monitors lambda errors"
  alarm_actions       = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  dimensions = {
    FunctionName = aws_lambda_function.backup_function.function_name
  }

  tags = {
    Name        = "${var.function_name}-errors"
    Environment = var.environment
    Project     = "db-backuper"
  }
}

# CloudWatch alarm for Lambda duration
resource "aws_cloudwatch_metric_alarm" "lambda_duration" {
  alarm_name          = "${var.function_name}-duration"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "Duration"
  namespace           = "AWS/Lambda"
  period              = "300"
  statistic           = "Average"
  threshold           = "600000" # 10 minutes in milliseconds
  alarm_description   = "This metric monitors lambda duration"
  alarm_actions       = var.alarm_sns_topic_arn != "" ? [var.alarm_sns_topic_arn] : []

  dimensions = {
    FunctionName = aws_lambda_function.backup_function.function_name
  }

  tags = {
    Name        = "${var.function_name}-duration"
    Environment = var.environment
    Project     = "db-backuper"
  }
}
