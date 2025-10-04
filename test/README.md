# Test Suite for PostgreSQL Database Backup Service

This directory contains comprehensive tests for the PostgreSQL database backup service, including integration tests with dockerized databases and AWS S3 (using LocalStack).

## Test Environment

The test suite uses Docker Compose to create a complete test environment with:

- **2 PostgreSQL Databases**: `testdb1` and `testdb2` running on ports 5433 and 5434
- **LocalStack**: AWS S3 emulator running on port 4566
- **Test Data**: Pre-configured tables and sample data

## Test Structure

### Files

- `setup_test.go`: Test database setup, connection management, and test data creation
- `backup_test.go`: Main test suite with backup creation, verification, and cleanup tests
- `run_tests.sh`: Test runner script that sets up environment and runs all tests
- `appsettings.test.json`: Configuration for S3-based tests
- `appsettings.test.local.json`: Configuration for local storage tests
- `unit/standalone_test.go`: Unit tests that don't require Docker services

### Test Categories

1. **Unit Tests** (No Docker required)
   - Configuration validation
   - Local storage operations
   - Backup cleanup functionality

2. **Backup Creation Tests** (Docker required)
   - Local storage backup creation and verification
   - S3 storage backup creation and verification
   - Data integrity verification

3. **Cleanup Tests** (Docker required)
   - Local storage cleanup of old backups
   - S3 storage cleanup of old backups
   - Retention policy enforcement

4. **Integration Tests** (Docker required)
   - Full end-to-end backup process
   - Multiple database backup verification
   - Error handling and recovery

## Running Tests

### Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- AWS CLI installed (for S3 bucket creation)

### Quick Start

```bash
# Run unit tests only (fast, no Docker required)
make test-unit

# Run all tests with Docker (comprehensive)
make test-run

# Or run the test script directly
./test/run_tests.sh
```

### Manual Test Execution

```bash
# 1. Setup test environment
make test-setup

# 2. Run tests
cd test
go test -v -timeout 10m

# 3. Cleanup
make test-clean
```

### Individual Test Commands

```bash
# Run unit tests only (no Docker required)
make test-unit

# Setup test environment only
make test-setup

# Run basic Go tests
make test

# Cleanup test environment
make test-clean
```

## Test Data

The test suite creates the following test data in each database:

### Tables

1. **test_users**
   - `id` (SERIAL PRIMARY KEY)
   - `name` (VARCHAR(100))
   - `email` (VARCHAR(100) UNIQUE)
   - `created_at` (TIMESTAMP)

2. **test_products**
   - `id` (SERIAL PRIMARY KEY)
   - `name` (VARCHAR(100))
   - `price` (DECIMAL(10,2))
   - `description` (TEXT)
   - `created_at` (TIMESTAMP)

### Sample Data

- **Users**: John Doe, Jane Smith, Bob Johnson
- **Products**: Laptop ($999.99), Mouse ($29.99), Keyboard ($79.99)

## Test Verification

The tests verify:

1. **Backup Creation**: Backups are created successfully for both databases
2. **Data Integrity**: Backup files contain expected tables and data
3. **File Organization**: Backups are stored in correct directory structure
4. **Cleanup Functionality**: Old backups are deleted according to retention policy
5. **Error Handling**: Service handles failures gracefully

## Test Configuration

### Database Configuration

```json
{
  "databases": [
    {
      "host": "localhost",
      "port": 5433,
      "username": "testuser",
      "password": "testpass",
      "database": "testdb1",
      "ssl_mode": "disable"
    },
    {
      "host": "localhost",
      "port": 5434,
      "username": "testuser",
      "password": "testpass",
      "database": "testdb2",
      "ssl_mode": "disable"
    }
  ]
}
```

### Storage Configuration

- **Local Storage**: `/tmp/test-backups`
- **S3 Storage**: `test-backup-bucket` (LocalStack)
- **Retention**: 1 day for testing

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 5433, 5434, and 4566 are available
2. **Docker Issues**: Make sure Docker is running and has sufficient resources
3. **Permission Issues**: Ensure the test script has execute permissions

### Debug Mode

To run tests with verbose output:

```bash
cd test
go test -v -timeout 10m
```

### Cleanup Issues

If tests fail to cleanup properly:

```bash
# Force cleanup
docker-compose -f docker-compose.test.yml down -v
docker system prune -f
rm -rf /tmp/test-backups
```

## Test Results

Successful test run should show:

```
=== RUN   TestBackupCreation
=== RUN   TestBackupCreation/LocalStorage
=== RUN   TestBackupCreation/S3Storage
=== RUN   TestBackupCleanup
=== RUN   TestBackupCleanup/LocalStorage
=== RUN   TestBackupCleanup/S3Storage
=== RUN   TestIntegration
=== RUN   TestIntegration/LocalStorage
=== RUN   TestIntegration/S3Storage
--- PASS: TestBackupCreation (X.XXs)
--- PASS: TestBackupCleanup (X.XXs)
--- PASS: TestIntegration (X.XXs)
PASS
ok      db-backuper/test    X.XXs
```

## Contributing

When adding new tests:

1. Follow the existing test structure
2. Add appropriate test data setup
3. Include cleanup in test teardown
4. Update this documentation
5. Ensure tests are deterministic and isolated
