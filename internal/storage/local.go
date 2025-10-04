package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"db-backuper/internal/config"

	"github.com/sirupsen/logrus"
)

// LocalStorage handles local file system operations
type LocalStorage struct {
	config *config.LocalConfig
	logger *logrus.Logger
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(localConfig *config.LocalConfig, logger *logrus.Logger) (*LocalStorage, error) {
	// Ensure the backup directory exists
	if err := os.MkdirAll(localConfig.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory %s: %w", localConfig.Path, err)
	}

	return &LocalStorage{
		config: localConfig,
		logger: logger,
	}, nil
}

// SaveBackup saves a backup file to local storage
func (ls *LocalStorage) SaveBackup(localFilePath, backupPrefix, databaseName string) (string, error) {
	filename := filepath.Base(localFilePath)

	// Create database-specific and date-based directory structure
	dateDir := time.Now().Format("2006-01-02")
	backupDir := filepath.Join(ls.config.Path, backupPrefix, databaseName, dateDir)

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory %s: %w", backupDir, err)
	}

	// Generate final backup path
	finalBackupPath := filepath.Join(backupDir, filename)

	// Copy the file to the final location
	if err := ls.copyFile(localFilePath, finalBackupPath); err != nil {
		return "", fmt.Errorf("failed to copy backup file: %w", err)
	}

	ls.logger.Infof("Backup saved to local storage: %s", finalBackupPath)
	return finalBackupPath, nil
}

// DeleteOldBackups deletes backup files older than the specified retention period
func (ls *LocalStorage) DeleteOldBackups(backupPrefix string, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	backupBaseDir := filepath.Join(ls.config.Path, backupPrefix)

	ls.logger.Infof("Deleting backups older than %d days (before %s)", retentionDays, cutoffDate.Format("2006-01-02"))

	// Check if backup directory exists
	if _, err := os.Stat(backupBaseDir); os.IsNotExist(err) {
		ls.logger.Info("Backup directory does not exist, nothing to clean up")
		return nil
	}

	// Read the backup directory to find database directories
	entries, err := os.ReadDir(backupBaseDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	var totalDeletedCount int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// This is a database directory
		databaseName := entry.Name()
		databaseDir := filepath.Join(backupBaseDir, databaseName)

		// Read date directories within the database directory
		dateEntries, err := os.ReadDir(databaseDir)
		if err != nil {
			ls.logger.Warnf("Failed to read database directory %s: %v", databaseName, err)
			continue
		}

		var deletedCount int
		for _, dateEntry := range dateEntries {
			if !dateEntry.IsDir() {
				continue
			}

			// Parse date from directory name (YYYY-MM-DD format)
			dirName := dateEntry.Name()
			if len(dirName) != 10 || strings.Count(dirName, "-") != 2 {
				ls.logger.Warnf("Skipping directory with invalid date format: %s", dirName)
				continue
			}

			dirDate, err := time.Parse("2006-01-02", dirName)
			if err != nil {
				ls.logger.Warnf("Failed to parse date from directory %s: %v", dirName, err)
				continue
			}

			// Check if directory is older than retention period
			if dirDate.Before(cutoffDate) {
				dirPath := filepath.Join(databaseDir, dirName)
				ls.logger.Infof("Deleting old backup directory: %s", dirPath)

				if err := os.RemoveAll(dirPath); err != nil {
					ls.logger.Errorf("Failed to delete directory %s: %v", dirPath, err)
					continue
				}

				deletedCount++
			}
		}

		ls.logger.Infof("Deleted %d old backup directories for database %s", deletedCount, databaseName)
		totalDeletedCount += deletedCount
	}

	ls.logger.Infof("Total deleted %d old backup directories across all databases", totalDeletedCount)
	return nil
}

// TestConnection tests the local storage connection
func (ls *LocalStorage) TestConnection() error {
	// Test if we can write to the backup directory
	testFile := filepath.Join(ls.config.Path, ".test-write")

	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("failed to create test file in backup directory: %w", err)
	}
	file.Close()

	// Clean up test file
	if err := os.Remove(testFile); err != nil {
		ls.logger.Warnf("Failed to remove test file: %v", err)
	}

	ls.logger.Info("Local storage connection test successful")
	return nil
}

// copyFile copies a file from src to dst
func (ls *LocalStorage) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy file contents
	_, err = destFile.ReadFrom(sourceFile)
	return err
}
