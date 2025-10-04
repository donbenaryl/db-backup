# PostgreSQL Database Backup Service

A Go-based service that automatically backs up multiple PostgreSQL databases to local storage or AWS S3 with configurable retention policies.

## Features

- **Automatic PostgreSQL backups** using `pg_dump`
- **Multiple database support** with individual configuration
- **Flexible storage options**: Local filesystem or AWS S3
- **Database-specific folders** for organized backup storage
- **Configurable retention policy** (default: 7 days)
- **Scheduled backups** using cron expressions
- **One-time backup** option
- **Connection testing** before running backups
- **Comprehensive logging** with configurable levels
- **Docker support** for easy deployment

## Configuration

All configuration is managed through `appsettings.json`. You must configure either local storage OR AWS S3 (not both):

**Local Storage Configuration:**
```json
{
  "databases": [
    {
      "host": "localhost",
      "port": 5432,
      "username": "postgres",
      "password": "password",
      "database": "mydb1",
      "ssl_mode": "disable"
    },
    {
      "host": "localhost",
      "port": 5432,
      "username": "postgres",
      "password": "password",
      "database": "mydb2",
      "ssl_mode": "disable"
    }
  ],
  "local": {
    "path": "/backups"
  },
  "backup": {
    "retention_days": 7,
    "schedule": "0 2 * * *",
    "backup_prefix": "postgres-backup"
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

**AWS S3 Configuration:**
```json
{
  "databases": [
    {
      "host": "localhost",
      "port": 5432,
      "username": "postgres",
      "password": "password",
      "database": "mydb1",
      "ssl_mode": "disable"
    },
    {
      "host": "localhost",
      "port": 5432,
      "username": "postgres",
      "password": "password",
      "database": "mydb2",
      "ssl_mode": "disable"
    }
  ],
  "aws": {
    "region": "us-east-1",
    "bucket": "my-backup-bucket",
    "access_key_id": "your-access-key-id",
    "secret_access_key": "your-secret-access-key"
  },
  "backup": {
    "retention_days": 7,
    "schedule": "0 2 * * *",
    "backup_prefix": "postgres-backup"
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

### Configuration Options

#### Database Configuration
The `databases` array contains one or more database configurations:
- `host`: PostgreSQL server hostname
- `port`: PostgreSQL server port (default: 5432)
- `username`: Database username
- `password`: Database password
- `database`: Database name to backup
- `ssl_mode`: SSL mode (disable, require, verify-full, etc.)

Each database can have different connection settings, allowing you to backup databases from different servers or with different credentials.

#### Local Storage Configuration
- `path`: Local directory path for storing backups

#### AWS Configuration
- `region`: AWS region for S3 bucket
- `bucket`: S3 bucket name for storing backups
- `access_key_id`: AWS access key ID
- `secret_access_key`: AWS secret access key

#### Backup Configuration
- `retention_days`: Number of days to keep backups (default: 7)
- `schedule`: Cron expression for scheduled backups (default: daily at 2 AM)
- `backup_prefix`: Prefix for backup files (used for both local and S3 storage)

#### Logging Configuration
- `level`: Log level (debug, info, warn, error)
- `format`: Log format (json, text)

## Usage

### Prerequisites

- Go 1.21 or later
- PostgreSQL client tools (`pg_dump`)
- AWS credentials with S3 access

### Running the Service

#### One-time Backup
```bash
go run ./cmd/main.go -once
```

#### Scheduled Backups
```bash
go run ./cmd/main.go
```

#### Custom Configuration
```bash
# For local storage
go run ./cmd/main.go -config appsettings.local.json

# For AWS S3 storage
go run ./cmd/main.go -config appsettings.aws.json

# Custom configuration file
go run ./cmd/main.go -config /path/to/custom-config.json
```

### Docker Usage

#### Build and Run
```bash
docker build -t db-backuper .
docker run -v $(pwd)/appsettings.json:/root/appsettings.json:ro db-backuper
```

#### Using Docker Compose
```bash
docker-compose up -d
```

The docker-compose file includes a test PostgreSQL instance for development.

## Backup File Organization

### Local Storage
Backups are organized locally with database-specific folders:
```
/backup-path/
└── backup-prefix/
    ├── database1/
    │   └── YYYY-MM-DD/
    │       └── database1_YYYY-MM-DD_HH-MM-SS.sql
    └── database2/
        └── YYYY-MM-DD/
            └── database2_YYYY-MM-DD_HH-MM-SS.sql
```

Example:
```
/backups/
└── postgres-backup/
    ├── mydb1/
    │   └── 2024-01-15/
    │       └── mydb1_2024-01-15_14-30-25.sql
    └── mydb2/
        └── 2024-01-15/
            └── mydb2_2024-01-15_14-30-25.sql
```

### AWS S3 Storage
Backups are organized in S3 with database-specific folders:
```
s3://bucket-name/
└── backup-prefix/
    ├── database1/
    │   └── YYYY-MM-DD/
    │       └── database1_YYYY-MM-DD_HH-MM-SS.sql
    └── database2/
        └── YYYY-MM-DD/
            └── database2_YYYY-MM-DD_HH-MM-SS.sql
```

Example:
```
s3://my-backup-bucket/
└── postgres-backup/
    ├── mydb1/
    │   └── 2024-01-15/
    │       └── mydb1_2024-01-15_14-30-25.sql
    └── mydb2/
        └── 2024-01-15/
            └── mydb2_2024-01-15_14-30-25.sql
```

## Retention Policy

The service automatically deletes backup files older than the configured retention period. By default, backups older than 7 days are removed.

## Logging

The service provides comprehensive logging with configurable levels and formats:

- **JSON format**: Structured logging for production environments
- **Text format**: Human-readable logs for development
- **Multiple levels**: Debug, Info, Warn, Error

## Error Handling

The service includes robust error handling:

- Connection testing before backup operations
- Automatic cleanup of local backup files
- Graceful handling of S3 upload failures
- Detailed error logging

## Security Considerations

- Store AWS credentials securely (consider using IAM roles in production)
- Use SSL/TLS for database connections in production
- Restrict S3 bucket permissions to minimum required access
- Consider using AWS Secrets Manager for credential management

## Development

### Project Structure
```
db-backuper/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── backup/
│   │   └── postgres.go      # PostgreSQL backup logic
│   └── s3/
│       └── s3.go           # AWS S3 operations
├── appsettings.json        # Configuration file
├── Dockerfile             # Docker configuration
├── docker-compose.yml     # Docker Compose setup
├── go.mod                 # Go module definition
└── README.md             # This file
```

### Building
```bash
go build -o db-backuper ./cmd/main.go
```

### Testing
```bash
go test ./...
```

## License

This project is licensed under the MIT License.
