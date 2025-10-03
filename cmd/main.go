package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
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
	postgresBackup := backup.NewPostgresBackup(&cfg.Database, logger)

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
	if err := testConnections(postgresBackup, storageManager, logger); err != nil {
		logger.Fatalf("Connection test failed: %v", err)
	}

	if *runOnce {
		// Run backup once and exit
		if err := performBackup(postgresBackup, storageManager, &cfg.Backup, logger); err != nil {
			logger.Fatalf("Backup failed: %v", err)
		}
		logger.Info("Backup completed successfully")
		return
	}

	// Setup scheduled backups
	c := cron.New()
	_, err = c.AddFunc(cfg.Backup.Schedule, func() {
		if err := performBackup(postgresBackup, storageManager, &cfg.Backup, logger); err != nil {
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
func testConnections(postgresBackup *backup.PostgresBackup, storageManager interface{}, logger *logrus.Logger) error {
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

	// Test database connection by attempting to create a backup
	logger.Info("Testing database connection...")
	backupPath, err := postgresBackup.CreateBackup()
	if err != nil {
		return fmt.Errorf("database connection test failed: %w", err)
	}

	// Cleanup test backup
	if err := postgresBackup.CleanupBackup(backupPath); err != nil {
		logger.Warnf("Failed to cleanup test backup: %v", err)
	}

	logger.Info("All connection tests passed")
	return nil
}

// performBackup performs a complete backup operation
func performBackup(postgresBackup *backup.PostgresBackup, storageManager interface{}, backupConfig *config.BackupConfig, logger *logrus.Logger) error {
	startTime := time.Now()
	logger.Info("Starting backup operation")

	// Create database backup
	backupPath, err := postgresBackup.CreateBackup()
	if err != nil {
		return fmt.Errorf("failed to create database backup: %w", err)
	}

	// Save backup to storage
	var finalPath string
	switch sm := storageManager.(type) {
	case *s3.S3Manager:
		s3Key, err := sm.UploadBackup(backupPath, backupConfig.BackupPrefix)
		if err != nil {
			// Cleanup local backup file on upload failure
			if cleanupErr := postgresBackup.CleanupBackup(backupPath); cleanupErr != nil {
				logger.Warnf("Failed to cleanup backup file after upload failure: %v", cleanupErr)
			}
			return fmt.Errorf("failed to upload backup to S3: %w", err)
		}
		finalPath = s3Key
	case *storage.LocalStorage:
		localPath, err := sm.SaveBackup(backupPath, backupConfig.BackupPrefix)
		if err != nil {
			// Cleanup local backup file on save failure
			if cleanupErr := postgresBackup.CleanupBackup(backupPath); cleanupErr != nil {
				logger.Warnf("Failed to cleanup backup file after save failure: %v", cleanupErr)
			}
			return fmt.Errorf("failed to save backup to local storage: %w", err)
		}
		finalPath = localPath
	default:
		return fmt.Errorf("unknown storage manager type")
	}

	// Cleanup local backup file
	if err := postgresBackup.CleanupBackup(backupPath); err != nil {
		logger.Warnf("Failed to cleanup local backup file: %v", err)
	}

	// Cleanup old backups
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
	logger.Infof("Backup operation completed successfully in %v. Final path: %s", duration, finalPath)
	return nil
}
