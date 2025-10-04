package restore

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"db-backuper/internal/config"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

// PostgresImport handles PostgreSQL database import operations
type PostgresImport struct {
	config *config.ImportConfig
	logger *logrus.Logger
}

// NewPostgresImport creates a new PostgreSQL import instance
func NewPostgresImport(importConfig *config.ImportConfig, logger *logrus.Logger) *PostgresImport {
	return &PostgresImport{
		config: importConfig,
		logger: logger,
	}
}

// ImportBackup imports a backup file to the target database
func (pi *PostgresImport) ImportBackup() error {
	// Validate backup file exists
	if _, err := os.Stat(pi.config.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", pi.config.BackupPath)
	}

	pi.logger.Infof("Starting import of backup: %s", pi.config.BackupPath)
	pi.logger.Infof("Target database: %s@%s:%d/%s",
		pi.config.TargetDatabase.Username,
		pi.config.TargetDatabase.Host,
		pi.config.TargetDatabase.Port,
		pi.config.TargetDatabase.Database)

	// Test database connection
	if err := pi.testConnection(); err != nil {
		return fmt.Errorf("failed to connect to target database: %w", err)
	}

	// Drop existing database if requested
	if pi.config.DropExisting {
		if err := pi.dropDatabase(); err != nil {
			return fmt.Errorf("failed to drop existing database: %w", err)
		}
	}

	// Import the backup
	if err := pi.importBackupFile(); err != nil {
		return fmt.Errorf("failed to import backup: %w", err)
	}

	pi.logger.Info("Import completed successfully")
	return nil
}

// testConnection tests the connection to the target database
func (pi *PostgresImport) testConnection() error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		pi.config.TargetDatabase.Host,
		pi.config.TargetDatabase.Port,
		pi.config.TargetDatabase.Username,
		pi.config.TargetDatabase.Password,
		pi.config.TargetDatabase.Database,
		pi.config.TargetDatabase.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	pi.logger.Info("Database connection test successful")
	return nil
}

// dropDatabase drops the existing database
func (pi *PostgresImport) dropDatabase() error {
	pi.logger.Warnf("Dropping existing database: %s", pi.config.TargetDatabase.Database)

	// Connect to postgres database to drop the target database
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		pi.config.TargetDatabase.Host,
		pi.config.TargetDatabase.Port,
		pi.config.TargetDatabase.Username,
		pi.config.TargetDatabase.Password,
		pi.config.TargetDatabase.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer db.Close()

	// Terminate existing connections to the target database
	terminateSQL := fmt.Sprintf(`
		SELECT pg_terminate_backend(pid)
		FROM pg_stat_activity
		WHERE datname = '%s' AND pid <> pg_backend_pid()`,
		pi.config.TargetDatabase.Database)

	if _, err := db.Exec(terminateSQL); err != nil {
		pi.logger.Warnf("Failed to terminate existing connections: %v", err)
	}

	// Drop the database
	dropSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", pi.config.TargetDatabase.Database)
	if _, err := db.Exec(dropSQL); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	// Create the database
	createSQL := fmt.Sprintf("CREATE DATABASE %s", pi.config.TargetDatabase.Database)
	if _, err := db.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	pi.logger.Info("Database dropped and recreated successfully")
	return nil
}

// importBackupFile imports the backup file using psql
func (pi *PostgresImport) importBackupFile() error {
	// Build psql command
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		pi.config.TargetDatabase.Host,
		pi.config.TargetDatabase.Port,
		pi.config.TargetDatabase.Username,
		pi.config.TargetDatabase.Password,
		pi.config.TargetDatabase.Database,
		pi.config.TargetDatabase.SSLMode)

	// Set PGPASSWORD environment variable
	env := os.Environ()
	env = append(env, fmt.Sprintf("PGPASSWORD=%s", pi.config.TargetDatabase.Password))

	// Build command
	cmd := exec.Command("psql", dsn, "-f", pi.config.BackupPath)
	cmd.Env = env

	// Set working directory to the backup file's directory
	cmd.Dir = filepath.Dir(pi.config.BackupPath)

	pi.logger.Infof("Executing import command: psql %s -f %s", dsn, pi.config.BackupPath)

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql command failed: %w\nOutput: %s", err, string(output))
	}

	pi.logger.Infof("Import command output: %s", string(output))
	return nil
}
