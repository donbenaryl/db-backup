.PHONY: build run test clean docker-build docker-run help

# Default target
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  run            - Run the application with default config"
	@echo "  run-local      - Run with local storage configuration"
	@echo "  run-aws        - Run with AWS S3 configuration"
	@echo "  run-once       - Run backup once and exit"
	@echo "  run-once-local - Run backup once with local storage"
	@echo "  run-once-aws   - Run backup once with AWS S3"
	@echo "  test           - Run tests"
	@echo "  clean          - Clean build artifacts"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  help           - Show this help message"

# Build the application
build:
	go build -o db-backuper ./cmd/main.go

# Run the application
run:
	go run ./cmd/main.go

# Run with local storage
run-local:
	go run ./cmd/main.go -config appsettings.local.json

# Run with AWS S3 storage
run-aws:
	go run ./cmd/main.go -config appsettings.aws.json

# Run backup once
run-once:
	go run ./cmd/main.go -once

# Run backup once with local storage
run-once-local:
	go run ./cmd/main.go -config appsettings.local.json -once

# Run backup once with AWS S3 storage
run-once-aws:
	go run ./cmd/main.go -config appsettings.aws.json -once

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f db-backuper
	rm -rf /tmp/db-backuper

# Build Docker image
docker-build:
	docker build -t db-backuper .

# Run with Docker Compose
docker-run:
	docker-compose up -d

# Stop Docker Compose
docker-stop:
	docker-compose down

# View logs
logs:
	docker-compose logs -f db-backuper
