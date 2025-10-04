# AWS Lambda Deployment Guide

This document provides a comprehensive guide for deploying the PostgreSQL Backup Service to AWS Lambda.

**Note:** This Lambda deployment is optimized for backup operations only. For import/restore operations, use the standalone application.

## Overview

The deployment creates the following AWS resources:

- **AWS Lambda Function** - Runs the backup service
- **EventBridge Rule** - Triggers the Lambda every 3 hours
- **S3 Bucket** - Stores backup files (protected from deletion)
- **IAM Role & Policies** - Secure access permissions
- **CloudWatch Logs** - Function logging and monitoring
- **CloudWatch Alarms** - Error and performance monitoring

## Quick Start

1. **Configure AWS CLI:**
   ```bash
   aws configure
   ```

2. **Copy and edit configuration:**
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   nano terraform.tfvars
   ```

3. **Deploy:**
   ```bash
   ./deploy.sh deploy
   ```

## Configuration

### Database Configuration

Configure your databases in `terraform.tfvars`:

```hcl
databases = [
  {
    host     = "your-db-host.example.com"
    port     = 5432
    username = "backup_user"
    password = "your-secure-password"
    database = "your_database"
    ssl_mode = "require"
  },
  {
    host     = "your-second-db.example.com"
    port     = 5432
    username = "backup_user"
    password = "your-secure-password"
    database = "your_second_database"
    ssl_mode = "require"
  }
]
```

### S3 Bucket Configuration

```hcl
backup_bucket_name = "my-postgres-backup-storage"
backup_retention_days = 2
backup_prefix = "postgres-backup"
```

### Lambda Configuration

```hcl
function_name = "postgres-backup-service"
log_level = "info"
log_retention_days = 14
```

## Security Features

### S3 Bucket Protection

The S3 bucket is configured with:
- **Lifecycle protection** - `prevent_destroy = true`
- **Versioning enabled** - Multiple versions of backup files
- **Server-side encryption** - AES256 encryption
- **Public access blocked** - No public access allowed
- **Lifecycle policy** - Automatic cleanup of old backups

### IAM Permissions

The Lambda function has minimal required permissions:
- **S3 access** - Read/write/delete objects in the backup bucket
- **CloudWatch Logs** - Write logs
- **No other permissions** - Principle of least privilege

### Environment Variables

Sensitive data is passed via environment variables:
- Database credentials
- AWS access keys (optional - can use IAM role)
- Configuration settings

## Monitoring

### CloudWatch Alarms

Two alarms are automatically created:

1. **Error Alarm**
   - Triggers on Lambda function errors
   - Sends notifications to SNS topic (if configured)

2. **Duration Alarm**
   - Triggers when function takes > 10 minutes
   - Helps identify performance issues

### Logs

Lambda function logs are available in CloudWatch Logs:
- **Log Group:** `/aws/lambda/{function-name}`
- **Retention:** Configurable (default: 14 days)
- **Format:** JSON structured logging

## Backup Schedule

The service runs automatically every 3 hours:
- **Schedule:** `rate(3 hours)`
- **Timezone:** UTC
- **Customizable:** Can be changed in `main.tf`

## Backup Storage

### S3 Structure

Backups are stored with the following structure:
```
s3://bucket-name/
├── postgres-backup/
│   ├── database1/
│   │   ├── 2025-01-01/
│   │   │   ├── database1_2025-01-01_00-00-00.sql
│   │   │   └── database1_2025-01-01_03-00-00.sql
│   │   └── 2025-01-02/
│   └── database2/
│       └── 2025-01-01/
```

### Retention Policy

- **Default retention:** 2 days
- **Configurable:** Set via `backup_retention_days`
- **Automatic cleanup:** Old backups are automatically deleted
- **Version cleanup:** Old versions are cleaned up after 30 days

## Cost Optimization

### Lambda Costs
- **Execution time:** ~2-5 minutes per backup
- **Memory:** 512MB (configurable)
- **Frequency:** Every 3 hours
- **Estimated cost:** $0.50-2.00/month per database

### S3 Costs
- **Storage:** Pay for backup file storage
- **Requests:** Minimal (only during backup/cleanup)
- **Estimated cost:** $0.10-0.50/month per database

### CloudWatch Costs
- **Logs:** Pay for log storage and ingestion
- **Alarms:** Free for basic alarms
- **Estimated cost:** $0.10-0.30/month

## Troubleshooting

### Common Issues

1. **Lambda Timeout**
   - **Cause:** Large database or slow network
   - **Solution:** Increase timeout in `main.tf`

2. **Database Connection Failed**
   - **Cause:** Network/VPC configuration
   - **Solution:** Configure VPC settings or check security groups

3. **S3 Upload Failed**
   - **Cause:** IAM permissions or bucket policy
   - **Solution:** Check IAM role permissions

4. **Environment Variables Not Set**
   - **Cause:** Terraform configuration issue
   - **Solution:** Check `terraform.tfvars` and variable definitions

### Debugging Steps

1. **Check Lambda logs:**
   ```bash
   ./deploy.sh logs
   ```

2. **Test Lambda function:**
   ```bash
   aws lambda invoke --function-name {function-name} response.json
   ```

3. **Check S3 bucket:**
   ```bash
   aws s3 ls s3://{bucket-name} --recursive
   ```

4. **Verify Terraform state:**
   ```bash
   terraform show
   ```

## Customization

### Modify Schedule

Edit `main.tf` to change the backup frequency:

```hcl
# Every hour
schedule_expression = "rate(1 hour)"

# Every 6 hours  
schedule_expression = "rate(6 hours)"

# Daily at 2 AM UTC
schedule_expression = "cron(0 2 * * ? *)"
```

### Add VPC Support

To access databases in a VPC, add VPC configuration:

```hcl
vpc_config {
  subnet_ids         = var.subnet_ids
  security_group_ids = var.security_group_ids
}
```

### Add SNS Notifications

Configure SNS topic for alarm notifications:

```hcl
alarm_sns_topic_arn = "arn:aws:sns:us-east-1:123456789012:backup-alerts"
```

## Maintenance

### Updating the Function

To update the Lambda function code:

1. **Modify the code**
2. **Run deployment:**
   ```bash
   ./deploy.sh deploy
   ```

### Scaling

The Lambda function automatically scales:
- **Concurrent executions:** Up to 1000 (AWS limit)
- **Memory:** Configurable (128MB - 10GB)
- **Timeout:** Configurable (up to 15 minutes)

### Backup Verification

To verify backups are working:

1. **Check CloudWatch logs** for successful executions
2. **List S3 objects** to see backup files
3. **Test restore** from a backup file

## Cleanup

### Destroy Resources

To remove all resources (except S3 bucket):

```bash
./deploy.sh destroy
```

### Manual S3 Cleanup

To manually delete the S3 bucket:

```bash
aws s3 rb s3://{bucket-name} --force
```

**Warning:** This will permanently delete all backup files!

## Support

For issues or questions:

1. Check the Lambda function logs
2. Review the Terraform plan and outputs
3. Verify AWS permissions and configuration
4. Check the main application documentation
5. Review this deployment guide

## Security Best Practices

1. **Use IAM roles** instead of access keys when possible
2. **Enable VPC** for database access if needed
3. **Use least privilege** IAM policies
4. **Enable CloudTrail** for audit logging
5. **Regular security reviews** of IAM policies
6. **Monitor CloudWatch alarms** for issues
7. **Keep Terraform state** in a secure backend
8. **Use environment-specific** configurations
