variable "aws_region" {
  description = "AWS region for resources"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name (e.g., dev, staging, prod)"
  type        = string
  default     = "prod"
}

variable "function_name" {
  description = "Name of the Lambda function"
  type        = string
  default     = "postgres-backup-service"
}

variable "backup_bucket_name" {
  description = "Name of the S3 bucket for storing backups"
  type        = string
  default     = "postgres-backup-storage"
}

variable "backup_retention_days" {
  description = "Number of days to retain backup files"
  type        = number
  default     = 2
}

variable "backup_prefix" {
  description = "Prefix for backup files in S3"
  type        = string
  default     = "postgres-backup"
}

variable "log_level" {
  description = "Log level for the application"
  type        = string
  default     = "info"
  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "Log level must be one of: debug, info, warn, error."
  }
}

variable "log_retention_days" {
  description = "Number of days to retain CloudWatch logs"
  type        = number
  default     = 14
}

variable "aws_access_key_id" {
  description = "AWS Access Key ID for S3 access (leave empty to use IAM role)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "aws_secret_access_key" {
  description = "AWS Secret Access Key for S3 access (leave empty to use IAM role)"
  type        = string
  default     = ""
  sensitive   = true
}

variable "alarm_sns_topic_arn" {
  description = "SNS topic ARN for CloudWatch alarms (optional)"
  type        = string
  default     = ""
}

# Database configuration variables
variable "databases" {
  description = "List of databases to backup"
  type = list(object({
    host     = string
    port     = number
    username = string
    password = string
    database = string
    ssl_mode = string
  }))
  default = []
}

