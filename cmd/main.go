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
	s3Manager, err := s3.NewS3Manager(&cfg.AWS, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize S3 manager: %v", err)
	}

	// Test connections
	if err := testConnections(postgresBackup, s3Manager, logger); err != nil {
		logger.Fatalf("Connection test failed: %v", err)
	}

	if *runOnce {
		// Run backup once and exit
		if err := performBackup(postgresBackup, s3Manager, &cfg.Backup, logger); err != nil {
			logger.Fatalf("Backup failed: %v", err)
		}
		logger.Info("Backup completed successfully")
		return
	}

	// Setup scheduled backups
	c := cron.New()
	_, err = c.AddFunc(cfg.Backup.Schedule, func() {
		if err := performBackup(postgresBackup, s3Manager, &cfg.Backup, logger); err != nil {
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

// testConnections tests database and S3 connections
func testConnections(postgresBackup *backup.PostgresBackup, s3Manager *s3.S3Manager, logger *logrus.Logger) error {
	logger.Info("Testing connections...")

	// Test S3 connection
	if err := s3Manager.TestConnection(); err != nil {
		return fmt.Errorf("S3 connection test failed: %w", err)
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
func performBackup(postgresBackup *backup.PostgresBackup, s3Manager *s3.S3Manager, backupConfig *config.BackupConfig, logger *logrus.Logger) error {
	startTime := time.Now()
	logger.Info("Starting backup operation")

	// Create database backup
	backupPath, err := postgresBackup.CreateBackup()
	if err != nil {
		return fmt.Errorf("failed to create database backup: %w", err)
	}

	// Upload to S3
	s3Key, err := s3Manager.UploadBackup(backupPath, backupConfig.BackupPrefix)
	if err != nil {
		// Cleanup local backup file on upload failure
		if cleanupErr := postgresBackup.CleanupBackup(backupPath); cleanupErr != nil {
			logger.Warnf("Failed to cleanup backup file after upload failure: %v", cleanupErr)
		}
		return fmt.Errorf("failed to upload backup to S3: %w", err)
	}

	// Cleanup local backup file
	if err := postgresBackup.CleanupBackup(backupPath); err != nil {
		logger.Warnf("Failed to cleanup local backup file: %v", err)
	}

	// Cleanup old backups
	if err := s3Manager.DeleteOldBackups(backupConfig.BackupPrefix, backupConfig.RetentionDays); err != nil {
		logger.Warnf("Failed to cleanup old backups: %v", err)
	}

	duration := time.Since(startTime)
	logger.Infof("Backup operation completed successfully in %v. S3 key: %s", duration, s3Key)
	return nil
}
