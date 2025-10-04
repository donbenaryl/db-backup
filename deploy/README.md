# AWS Lambda Deployment for PostgreSQL Backup Service

This directory contains Terraform configuration files to deploy the PostgreSQL Backup Service to AWS Lambda. The service will automatically run every 3 hours and store backup files in an S3 bucket.

**Note:** This Lambda deployment is optimized for backup operations only. For import/restore operations, use the standalone application.

## Features

- **AWS Lambda deployment** with automatic scaling
- **EventBridge scheduling** to run every 3 hours
- **S3 bucket** for storing backup files with lifecycle management
- **CloudWatch monitoring** with alarms for errors and duration
- **IAM roles and policies** for secure access
- **Environment variable configuration** for database connections
- **S3 bucket protection** - never deleted even on `terraform destroy`

## Prerequisites

1. **AWS CLI** configured with appropriate permissions
2. **Terraform** (version >= 1.0)
3. **Go** (for building the Lambda function)
4. **AWS Account** with permissions to create:
   - Lambda functions
   - S3 buckets
   - IAM roles and policies
   - EventBridge rules
   - CloudWatch logs and alarms

## Quick Start

1. **Copy the example configuration:**
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

2. **Edit the configuration:**
   ```bash
   nano terraform.tfvars
   ```

3. **Deploy the service:**
   ```bash
   ./deploy.sh deploy
   ```

## Configuration

### Required Variables

Edit `terraform.tfvars` with your specific configuration:

```hcl
# AWS Configuration
aws_region = "us-east-1"
environment = "prod"

# Lambda Function
function_name = "postgres-backup-service"
backup_bucket_name = "my-postgres-backup-storage"

# Database Configuration
databases = [
  {
    host     = "your-db-host.example.com"
    port     = 5432
    username = "backup_user"
    password = "your-secure-password"
    database = "your_database"
    ssl_mode = "require"
  }
]
```

### Environment Variables

The Lambda function uses environment variables for configuration. These are set automatically by Terraform:

- `AWS_REGION` - AWS region
- `AWS_BUCKET` - S3 bucket name for backups
- `BACKUP_RETENTION_DAYS` - Days to retain backups (default: 2)
- `BACKUP_PREFIX` - Prefix for backup files
- `LOG_LEVEL` - Log level (debug, info, warn, error)
- `LOG_FORMAT` - Log format (json, text)

For database configuration, use indexed environment variables:
- `DB_0_HOST`, `DB_0_PORT`, `DB_0_USERNAME`, `DB_0_PASSWORD`, `DB_0_DATABASE`, `DB_0_SSL_MODE`
- `DB_1_HOST`, `DB_1_PORT`, etc. (for additional databases)

## Deployment Commands

### Deploy the Service
```bash
./deploy.sh deploy
```

### Show Deployment Plan
```bash
./deploy.sh plan
```

### View Deployment Outputs
```bash
./deploy.sh output
```

### View Lambda Logs
```bash
./deploy.sh logs
```

### Destroy Resources (Preserves S3 Bucket)
```bash
./deploy.sh destroy
```

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   EventBridge   │───▶│   AWS Lambda     │───▶│   S3 Bucket     │
│   (Every 3h)    │    │   (Backup Func)  │    │   (Backups)     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   CloudWatch     │
                       │   (Logs & Alarms)│
                       └──────────────────┘
```

## S3 Bucket Configuration

The S3 bucket is configured with:

- **Versioning enabled** for backup file history
- **Server-side encryption** (AES256)
- **Lifecycle policy** for automatic cleanup of old backups
- **Public access blocked** for security
- **Protection from deletion** - never deleted even on `terraform destroy`

## Monitoring

### CloudWatch Alarms

Two alarms are created automatically:

1. **Error Alarm** - Triggers when Lambda function encounters errors
2. **Duration Alarm** - Triggers when Lambda function takes longer than 10 minutes

### Logs

Lambda function logs are stored in CloudWatch Logs under:
```
/aws/lambda/{function-name}
```

## Security

- **IAM roles** with minimal required permissions
- **S3 bucket** with public access blocked
- **Environment variables** for sensitive configuration
- **VPC support** can be added for database access

## Cost Optimization

- **Lambda** - Pay only for execution time (every 3 hours)
- **S3** - Pay for storage and requests
- **CloudWatch** - Pay for logs and alarms
- **EventBridge** - Free for scheduled events

## Troubleshooting

### Common Issues

1. **Lambda timeout** - Increase timeout in `main.tf`
2. **Database connection** - Check security groups and VPC configuration
3. **S3 permissions** - Verify IAM role has S3 access
4. **Environment variables** - Check Lambda function configuration

### Debugging

1. **Check Lambda logs:**
   ```bash
   ./deploy.sh logs
   ```

2. **Test Lambda function manually:**
   ```bash
   aws lambda invoke --function-name {function-name} response.json
   ```

3. **Check S3 bucket contents:**
   ```bash
   aws s3 ls s3://{bucket-name} --recursive
   ```

## Customization

### Modify Schedule

To change the backup frequency, update the `schedule_expression` in `main.tf`:

```hcl
# Every hour
schedule_expression = "rate(1 hour)"

# Every 6 hours
schedule_expression = "rate(6 hours)"

# Daily at 2 AM UTC
schedule_expression = "cron(0 2 * * ? *)"
```

### Add VPC Configuration

To access databases in a VPC, add VPC configuration to the Lambda function:

```hcl
vpc_config {
  subnet_ids         = var.subnet_ids
  security_group_ids = var.security_group_ids
}
```

### Add SNS Notifications

To receive notifications for alarms, set the `alarm_sns_topic_arn` variable:

```hcl
alarm_sns_topic_arn = "arn:aws:sns:us-east-1:123456789012:backup-alerts"
```

## Cleanup

To remove all resources (except the S3 bucket):

```bash
./deploy.sh destroy
```

**Note:** The S3 bucket will be preserved for data safety. To manually delete it:

```bash
aws s3 rb s3://{bucket-name} --force
```

## Support

For issues or questions:

1. Check the Lambda function logs
2. Review the Terraform plan and outputs
3. Verify AWS permissions and configuration
4. Check the main application documentation
