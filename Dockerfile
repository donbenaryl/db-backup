# Build stage
FROM golang:1.21-alpine AS builder

# Install postgresql-client for pg_dump
RUN apk add --no-cache postgresql-client

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/main.go

# Final stage
FROM alpine:latest

# Install postgresql-client for pg_dump
RUN apk add --no-cache postgresql-client ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy configuration file
COPY --from=builder /app/appsettings.json .

# Create directory for temporary backups
RUN mkdir -p /tmp/db-backuper

# Run the application
CMD ["./main"]
