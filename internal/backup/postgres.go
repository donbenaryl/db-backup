package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"db-backuper/internal/config"

	"github.com/sirupsen/logrus"
)

// PostgresBackup handles PostgreSQL database backups
type PostgresBackup struct {
	config *config.DatabaseConfig
	logger *logrus.Logger
}

// NewPostgresBackup creates a new PostgreSQL backup instance
func NewPostgresBackup(dbConfig *config.DatabaseConfig, logger *logrus.Logger) *PostgresBackup {
	return &PostgresBackup{
		config: dbConfig,
		logger: logger,
	}
}

// CreateBackup creates a PostgreSQL database backup
func (pb *PostgresBackup) CreateBackup() (string, error) {
	// Generate backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFilename := fmt.Sprintf("%s_%s.sql", pb.config.Database, timestamp)

	// Create temporary directory for backup
	tempDir := "/tmp/db-backuper"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	backupPath := filepath.Join(tempDir, backupFilename)

	// Set PGPASSWORD environment variable for pg_dump
	env := os.Environ()
	env = append(env, fmt.Sprintf("PGPASSWORD=%s", pb.config.Password))

	// Build pg_dump command
	cmd := exec.Command("pg_dump",
		"-h", pb.config.Host,
		"-p", fmt.Sprintf("%d", pb.config.Port),
		"-U", pb.config.Username,
		"-d", pb.config.Database,
		"-f", backupPath,
		"--verbose",
		"--no-password",
	)

	cmd.Env = env

	pb.logger.Infof("Creating backup: %s", backupPath)

	// Execute pg_dump
	output, err := cmd.CombinedOutput()
	if err != nil {
		pb.logger.Errorf("pg_dump failed: %s", string(output))
		return "", fmt.Errorf("pg_dump failed: %w", err)
	}

	pb.logger.Infof("Backup created successfully: %s", backupPath)
	return backupPath, nil
}

// CleanupBackup removes the local backup file
func (pb *PostgresBackup) CleanupBackup(backupPath string) error {
	if err := os.Remove(backupPath); err != nil {
		pb.logger.Warnf("Failed to cleanup backup file %s: %v", backupPath, err)
		return err
	}

	pb.logger.Infof("Cleaned up backup file: %s", backupPath)
	return nil
}
