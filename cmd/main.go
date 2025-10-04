package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"db-backuper/internal/backup"
	"db-backuper/internal/config"
	"db-backuper/internal/s3"
	"db-backuper/internal/storage"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "appsettings.json", "Path to configuration file")
	runOnce := flag.Bool("once", false, "Run backup once and exit")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg.Logging)
	logger.Info("Starting PostgreSQL backup service")

	// Initialize backup components
	postgresBackups := make([]*backup.PostgresBackup, len(cfg.Databases))
	for i, dbConfig := range cfg.Databases {
		postgresBackups[i] = backup.NewPostgresBackup(&dbConfig, logger)
	}

	var storageManager interface{}
	if cfg.IsLocalStorage() {
		localStorage, err := storage.NewLocalStorage(&cfg.Local, logger)
		if err != nil {
			logger.Fatalf("Failed to initialize local storage: %v", err)
		}
		storageManager = localStorage
		logger.Info("Using local storage for backups")
	} else if cfg.IsAWSStorage() {
		s3Manager, err := s3.NewS3Manager(&cfg.AWS, logger)
		if err != nil {
			logger.Fatalf("Failed to initialize S3 manager: %v", err)
		}
		storageManager = s3Manager
		logger.Info("Using AWS S3 for backups")
	}

	// Test connections
	if err := testConnections(postgresBackups, storageManager, logger); err != nil {
		logger.Fatalf("Connection test failed: %v", err)
	}

	if *runOnce {
		// Run backup once and exit
		if err := performBackup(postgresBackups, storageManager, &cfg.Backup, logger); err != nil {
			logger.Fatalf("Backup failed: %v", err)
		}
		logger.Info("Backup completed successfully")
		return
	}

	// Setup scheduled backups
	c := cron.New()
	_, err = c.AddFunc(cfg.Backup.Schedule, func() {
		if err := performBackup(postgresBackups, storageManager, &cfg.Backup, logger); err != nil {
			logger.Errorf("Scheduled backup failed: %v", err)
		}
	})
	if err != nil {
		logger.Fatalf("Failed to schedule backup: %v", err)
	}

	logger.Infof("Scheduled backup with cron expression: %s", cfg.Backup.Schedule)
	c.Start()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down backup service")
	c.Stop()
}

// setupLogger configures the logger based on configuration
func setupLogger(loggingConfig config.LoggingConfig) *logrus.Logger {
	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(loggingConfig.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)

	// Set log format
	if loggingConfig.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	return logger
}

// testConnections tests database and storage connections
func testConnections(postgresBackups []*backup.PostgresBackup, storageManager interface{}, logger *logrus.Logger) error {
	logger.Info("Testing connections...")

	// Test storage connection
	switch sm := storageManager.(type) {
	case *s3.S3Manager:
		if err := sm.TestConnection(); err != nil {
			return fmt.Errorf("S3 connection test failed: %w", err)
		}
	case *storage.LocalStorage:
		if err := sm.TestConnection(); err != nil {
			return fmt.Errorf("local storage connection test failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown storage manager type")
	}

	// Test database connections by attempting to create a backup for each database
	logger.Info("Testing database connections...")
	for i, postgresBackup := range postgresBackups {
		logger.Infof("Testing connection for database %d...", i+1)
		backupPath, err := postgresBackup.CreateBackup()
		if err != nil {
			return fmt.Errorf("database %d connection test failed: %w", i+1, err)
		}

		// Cleanup test backup
		if err := postgresBackup.CleanupBackup(backupPath); err != nil {
			logger.Warnf("Failed to cleanup test backup for database %d: %v", i+1, err)
		}
	}

	logger.Info("All connection tests passed")
	return nil
}

// performBackup performs a complete backup operation for all databases
func performBackup(postgresBackups []*backup.PostgresBackup, storageManager interface{}, backupConfig *config.BackupConfig, logger *logrus.Logger) error {
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

		// Save backup to storage
		var finalPath string
		switch sm := storageManager.(type) {
		case *s3.S3Manager:
			s3Key, err := sm.UploadBackup(backupPath, backupConfig.BackupPrefix, databaseName)
			if err != nil {
				// Cleanup local backup file on upload failure
				if cleanupErr := postgresBackup.CleanupBackup(backupPath); cleanupErr != nil {
					logger.Warnf("Failed to cleanup backup file after upload failure: %v", cleanupErr)
				}
				logger.Errorf("Failed to upload backup for database %d to S3: %v", i+1, err)
				failedBackups++
				continue
			}
			finalPath = s3Key
		case *storage.LocalStorage:
			localPath, err := sm.SaveBackup(backupPath, backupConfig.BackupPrefix, databaseName)
			if err != nil {
				// Cleanup local backup file on save failure
				if cleanupErr := postgresBackup.CleanupBackup(backupPath); cleanupErr != nil {
					logger.Warnf("Failed to cleanup backup file after save failure: %v", cleanupErr)
				}
				logger.Errorf("Failed to save backup for database %d to local storage: %v", i+1, err)
				failedBackups++
				continue
			}
			finalPath = localPath
		default:
			logger.Errorf("Unknown storage manager type for database %d", i+1)
			failedBackups++
			continue
		}

		// Cleanup local backup file
		if err := postgresBackup.CleanupBackup(backupPath); err != nil {
			logger.Warnf("Failed to cleanup local backup file for database %d: %v", i+1, err)
		}

		logger.Infof("Successfully backed up database %d to: %s", i+1, finalPath)
		successfulBackups++
	}

	// Cleanup old backups (only once, not per database)
	logger.Info("Cleaning up old backups...")
	switch sm := storageManager.(type) {
	case *s3.S3Manager:
		if err := sm.DeleteOldBackups(backupConfig.BackupPrefix, backupConfig.RetentionDays); err != nil {
			logger.Warnf("Failed to cleanup old S3 backups: %v", err)
		}
	case *storage.LocalStorage:
		if err := sm.DeleteOldBackups(backupConfig.BackupPrefix, backupConfig.RetentionDays); err != nil {
			logger.Warnf("Failed to cleanup old local backups: %v", err)
		}
	}

	duration := time.Since(startTime)
	logger.Infof("Backup operation completed in %v. Successful: %d, Failed: %d", duration, successfulBackups, failedBackups)

	if failedBackups > 0 {
		return fmt.Errorf("backup operation completed with %d failures out of %d databases", failedBackups, len(postgresBackups))
	}

	return nil
}
