package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"db-backuper/internal/backup"
	"db-backuper/internal/config"
	"db-backuper/internal/s3"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/sirupsen/logrus"
)

// LambdaEvent represents the Lambda event structure
type LambdaEvent struct {
	// Simple event structure for backup-only Lambda
}

// LambdaResponse represents the Lambda response structure
type LambdaResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Success    bool   `json:"success"`
}

// Handler is the main Lambda handler function
func Handler(ctx context.Context, event LambdaEvent) (LambdaResponse, error) {
	// Setup logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.SetFormatter(&logrus.JSONFormatter{})

	logger.Info("Starting PostgreSQL backup Lambda function")

	// Log Lambda environment information for debugging
	logger.Infof("Lambda environment: PATH=%s", os.Getenv("PATH"))
	logger.Infof("Lambda runtime: %s", os.Getenv("AWS_EXECUTION_ENV"))
	logger.Infof("Lambda region: %s", os.Getenv("AWS_REGION"))

	// Load configuration
	cfg, err := loadLambdaConfig()
	if err != nil {
		logger.WithError(err).Error("Failed to load configuration")
		return LambdaResponse{
			StatusCode: 500,
			Message:    fmt.Sprintf("Configuration error: %v", err),
			Success:    false,
		}, nil
	}

	// Setup logger with configuration
	logger = setupLogger(cfg.Logging)

	// Execute backup operation
	return handleBackup(cfg, logger)
}

// loadLambdaConfig loads configuration for Lambda environment
func loadLambdaConfig() (*config.Config, error) {
	// Create a minimal config that will be populated by environment variables
	cfg := &config.Config{
		Databases: []config.DatabaseConfig{},
		AWS: config.AWSConfig{
			Region:          os.Getenv("AWS_REGION"), // Automatically provided by Lambda
			Bucket:          os.Getenv("AWS_BUCKET"),
			AccessKeyID:     "", // Not needed - Lambda uses IAM role
			SecretAccessKey: "", // Not needed - Lambda uses IAM role
		},
		Backup: config.BackupConfig{
			RetentionDays: 2,
			Schedule:      "0 */3 * * *",
			BackupPrefix:  "postgres-backup",
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	// Apply environment variable overrides
	if err := applyLambdaEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration for backup operations
	if err := cfg.ValidateForBackup(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// applyLambdaEnvOverrides applies environment variable overrides for Lambda
func applyLambdaEnvOverrides(cfg *config.Config) error {
	// Parse environment variables for each section
	if err := parseLambdaConfigSections(cfg); err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Handle database arrays from environment variables
	if err := parseLambdaDatabases(cfg); err != nil {
		return fmt.Errorf("failed to parse database environment variables: %w", err)
	}

	return nil
}

// parseLambdaConfigSections parses environment variables for different config sections
func parseLambdaConfigSections(cfg *config.Config) error {
	// Parse AWS config
	if region := os.Getenv("AWS_REGION"); region != "" {
		cfg.AWS.Region = region
	}
	if bucket := os.Getenv("AWS_BUCKET"); bucket != "" {
		cfg.AWS.Bucket = bucket
	}
	if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
		cfg.AWS.AccessKeyID = accessKey
	}
	if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
		cfg.AWS.SecretAccessKey = secretKey
	}

	// Parse Backup config
	if retentionDays := os.Getenv("BACKUP_RETENTION_DAYS"); retentionDays != "" {
		if days, err := parseInt(retentionDays); err == nil {
			cfg.Backup.RetentionDays = days
		}
	}
	if schedule := os.Getenv("BACKUP_SCHEDULE"); schedule != "" {
		cfg.Backup.Schedule = schedule
	}
	if prefix := os.Getenv("BACKUP_PREFIX"); prefix != "" {
		cfg.Backup.BackupPrefix = prefix
	}

	// Parse Logging config
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		cfg.Logging.Format = format
	}

	return nil
}

// parseLambdaDatabases parses database configuration from environment variables
func parseLambdaDatabases(cfg *config.Config) error {
	// Check for database configuration in environment variables
	// Format: DB_0_HOST, DB_0_PORT, DB_0_USERNAME, etc.
	i := 0
	for {
		host := os.Getenv(fmt.Sprintf("DB_%d_HOST", i))
		if host == "" {
			if i == 0 {
				return fmt.Errorf("no database configuration found - please set DB_0_HOST environment variable")
			}
			break // No more databases
		}

		db := config.DatabaseConfig{
			Host:     host,
			Port:     5432, // Default port
			Username: os.Getenv(fmt.Sprintf("DB_%d_USERNAME", i)),
			Password: os.Getenv(fmt.Sprintf("DB_%d_PASSWORD", i)),
			Database: os.Getenv(fmt.Sprintf("DB_%d_DATABASE", i)),
			SSLMode:  os.Getenv(fmt.Sprintf("DB_%d_SSL_MODE", i)),
		}

		// Parse port if provided
		if portStr := os.Getenv(fmt.Sprintf("DB_%d_PORT", i)); portStr != "" {
			if port, err := parseInt(portStr); err == nil {
				db.Port = port
			}
		}

		// Set default SSL mode if not provided
		if db.SSLMode == "" {
			db.SSLMode = "disable"
		}

		// Validate required fields
		if db.Username == "" {
			return fmt.Errorf("DB_%d_USERNAME is required", i)
		}
		if db.Password == "" {
			return fmt.Errorf("DB_%d_PASSWORD is required", i)
		}
		if db.Database == "" {
			return fmt.Errorf("DB_%d_DATABASE is required", i)
		}

		cfg.Databases = append(cfg.Databases, db)
		i++
	}

	return nil
}

// parseInt parses a string to integer
func parseInt(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}

// setupLogger configures the logger based on configuration
func setupLogger(loggingConfig config.LoggingConfig) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	switch loggingConfig.Level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set log format
	if loggingConfig.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	return logger
}

// handleBackup handles backup operations
func handleBackup(cfg *config.Config, logger *logrus.Logger) (LambdaResponse, error) {
	logger.Info("Starting backup operation")

	// Initialize S3 manager
	s3Manager, err := s3.NewS3Manager(&cfg.AWS, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to initialize S3 manager")
		return LambdaResponse{
			StatusCode: 500,
			Message:    fmt.Sprintf("S3 initialization error: %v", err),
			Success:    false,
		}, nil
	}

	// Create PostgreSQL backup instances for each database
	var postgresBackups []*backup.PostgresBackup
	for i, dbConfig := range cfg.Databases {
		logger.Infof("Initializing backup for database %d: %s", i+1, dbConfig.Database)
		postgresBackup := backup.NewPostgresBackup(&dbConfig, logger)

		// Test connection before adding to backup list
		if err := postgresBackup.TestConnection(); err != nil {
			logger.WithError(err).Errorf("Connection test failed for database %d", i+1)
			return LambdaResponse{
				StatusCode: 500,
				Message:    fmt.Sprintf("Database connection test failed for database %d: %v", i+1, err),
				Success:    false,
			}, nil
		}

		postgresBackups = append(postgresBackups, postgresBackup)
	}

	// Run backup using the same logic as the main application
	if err := performLambdaBackup(postgresBackups, s3Manager, &cfg.Backup, logger); err != nil {
		logger.WithError(err).Error("Backup operation failed")
		return LambdaResponse{
			StatusCode: 500,
			Message:    fmt.Sprintf("Backup failed: %v", err),
			Success:    false,
		}, nil
	}

	logger.Info("Backup operation completed successfully")
	return LambdaResponse{
		StatusCode: 200,
		Message:    "Backup completed successfully",
		Success:    true,
	}, nil
}

// performLambdaBackup performs backup operations for Lambda
func performLambdaBackup(postgresBackups []*backup.PostgresBackup, s3Manager *s3.S3Manager, backupConfig *config.BackupConfig, logger *logrus.Logger) error {
	startTime := time.Now()
	logger.Infof("Starting backup operation for %d databases", len(postgresBackups))

	var successfulBackups int
	var failedBackups int

	// Backup each database
	for i, postgresBackup := range postgresBackups {
		logger.Infof("Backing up database %d of %d", i+1, len(postgresBackups))

		// Create database backup
		backupPath, err := postgresBackup.CreateBackup()
		if err != nil {
			logger.Errorf("Failed to create backup for database %d: %v", i+1, err)
			failedBackups++
			continue
		}

		// Get database name from the backup path (it's in the filename)
		// Format: database-name_YYYY-MM-DD_HH-MM-SS.sql
		filename := filepath.Base(backupPath)
		databaseName := strings.Split(filename, "_")[0]

		// Save backup to S3
		s3Key, err := s3Manager.UploadBackup(backupPath, backupConfig.BackupPrefix, databaseName)
		if err != nil {
			// Cleanup local backup file on upload failure
			if cleanupErr := postgresBackup.CleanupBackup(backupPath); cleanupErr != nil {
				logger.Warnf("Failed to cleanup backup file after upload failure: %v", cleanupErr)
			}
			logger.Errorf("Failed to upload backup for database %d to S3: %v", i+1, err)
			failedBackups++
			continue
		}

		// Cleanup local backup file after successful upload
		if err := postgresBackup.CleanupBackup(backupPath); err != nil {
			logger.Warnf("Failed to cleanup backup file: %v", err)
		}

		logger.Infof("Successfully backed up database %d to: %s", i+1, s3Key)
		successfulBackups++
	}

	// Clean up old backups
	logger.Info("Cleaning up old backups...")
	if err := s3Manager.DeleteOldBackups(backupConfig.BackupPrefix, backupConfig.RetentionDays); err != nil {
		logger.Errorf("Failed to cleanup old backups: %v", err)
	}

	duration := time.Since(startTime)
	logger.Infof("Backup operation completed in %v. Successful: %d, Failed: %d", duration, successfulBackups, failedBackups)

	if failedBackups > 0 {
		return fmt.Errorf("backup operation completed with %d failures", failedBackups)
	}

	return nil
}

func main() {
	lambda.Start(Handler)
}
