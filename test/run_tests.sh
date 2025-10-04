#!/bin/bash

set -e

echo "Starting PostgreSQL Database Backup Service Tests"
echo "================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to cleanup on exit
cleanup() {
    print_status "Cleaning up test environment..."
    docker-compose -f docker-compose.test.yml down -v
    docker system prune -f
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    print_error "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Check if docker-compose is available
if ! command -v docker-compose > /dev/null 2>&1; then
    print_error "docker-compose is not installed. Please install docker-compose and try again."
    exit 1
fi

print_status "Starting test environment with Docker Compose..."

# Start test services
docker-compose -f docker-compose.test.yml up -d

print_status "Waiting for services to be ready..."

# Wait for PostgreSQL databases to be ready
print_status "Waiting for PostgreSQL test databases..."
for i in {1..30}; do
    if docker-compose -f docker-compose.test.yml exec -T postgres-test-1 pg_isready -U testuser -d testdb1 > /dev/null 2>&1 && \
       docker-compose -f docker-compose.test.yml exec -T postgres-test-2 pg_isready -U testuser -d testdb2 > /dev/null 2>&1; then
        print_status "PostgreSQL databases are ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        print_error "PostgreSQL databases failed to start within 60 seconds"
        exit 1
    fi
    sleep 2
done

# Wait for LocalStack to be ready
print_status "Waiting for LocalStack..."
for i in {1..30}; do
    if curl -f http://localhost:4566/health > /dev/null 2>&1; then
        print_status "LocalStack is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        print_error "LocalStack failed to start within 60 seconds"
        exit 1
    fi
    sleep 2
done

# Create S3 bucket for testing
print_status "Creating S3 test bucket..."
aws --endpoint-url=http://localhost:4566 s3 mb s3://test-backup-bucket --region us-east-1 || true

# Build the application
print_status "Building the application..."
go build -o db-backuper ./cmd/main.go

# Run tests
print_status "Running tests..."
cd test
RUN_INTEGRATION_TESTS=true go test -v -timeout 10m

# Check test results
if [ $? -eq 0 ]; then
    print_status "All tests passed! ✅"
else
    print_error "Some tests failed! ❌"
    exit 1
fi

print_status "Test run completed successfully!"
