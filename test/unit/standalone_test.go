package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"db-backuper/internal/config"
	"db-backuper/internal/storage"

	"github.com/sirupsen/logrus"
)

// TestConfigurationValidation tests configuration validation without database setup
func TestConfigurationValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "Valid configuration with local storage",
			config: &config.Config{
				Databases: []config.DatabaseConfig{
					{
						Host:     "localhost",
						Port:     5432,
						Username: "user",
						Password: "pass",
						Database: "testdb",
						SSLMode:  "disable",
					},
				},
				Local: config.LocalConfig{
					Path: "/tmp/backups",
				},
				Backup: config.BackupConfig{
					RetentionDays: 7,
					Schedule:      "0 2 * * *",
					BackupPrefix:  "test-backup",
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			expectError: false,
		},
		{
			name: "Valid configuration with AWS S3",
			config: &config.Config{
				Databases: []config.DatabaseConfig{
					{
						Host:     "localhost",
						Port:     5432,
						Username: "user",
						Password: "pass",
						Database: "testdb",
						SSLMode:  "disable",
					},
				},
				AWS: config.AWSConfig{
					Region:          "us-east-1",
					Bucket:          "test-bucket",
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
				},
				Backup: config.BackupConfig{
					RetentionDays: 7,
					Schedule:      "0 2 * * *",
					BackupPrefix:  "test-backup",
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			expectError: false,
		},
		{
			name: "No databases configured",
			config: &config.Config{
				Databases: []config.DatabaseConfig{},
				Local: config.LocalConfig{
					Path: "/tmp/backups",
				},
				Backup: config.BackupConfig{
					RetentionDays: 7,
					Schedule:      "0 2 * * *",
					BackupPrefix:  "test-backup",
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			expectError: true,
		},
		{
			name: "Both local and AWS configured",
			config: &config.Config{
				Databases: []config.DatabaseConfig{
					{
						Host:     "localhost",
						Port:     5432,
						Username: "user",
						Password: "pass",
						Database: "testdb",
						SSLMode:  "disable",
					},
				},
				Local: config.LocalConfig{
					Path: "/tmp/backups",
				},
				AWS: config.AWSConfig{
					Region:          "us-east-1",
					Bucket:          "test-bucket",
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
				},
				Backup: config.BackupConfig{
					RetentionDays: 7,
					Schedule:      "0 2 * * *",
					BackupPrefix:  "test-backup",
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			expectError: true,
		},
		{
			name: "No storage configured",
			config: &config.Config{
				Databases: []config.DatabaseConfig{
					{
						Host:     "localhost",
						Port:     5432,
						Username: "user",
						Password: "pass",
						Database: "testdb",
						SSLMode:  "disable",
					},
				},
				Backup: config.BackupConfig{
					RetentionDays: 7,
					Schedule:      "0 2 * * *",
					BackupPrefix:  "test-backup",
				},
				Logging: config.LoggingConfig{
					Level:  "info",
					Format: "json",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestLocalStorageOperations tests local storage operations without database setup
func TestLocalStorageOperations(t *testing.T) {
	// Create temporary directory for testing
	tempDir := "/tmp/test-local-storage"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create local storage instance
	localStorage, err := storage.NewLocalStorage(&config.LocalConfig{
		Path: tempDir,
	}, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Test connection
	if err := localStorage.TestConnection(); err != nil {
		t.Errorf("Test connection failed: %v", err)
	}

	// Create test backup file
	testContent := "-- Test backup content\nCREATE TABLE test (id INT);\nINSERT INTO test VALUES (1);"
	testFile := filepath.Join(tempDir, "test_backup.sql")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Save backup
	backupPath, err := localStorage.SaveBackup(testFile, "test-backup", "testdb")
	if err != nil {
		t.Fatalf("Failed to save backup: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file does not exist: %s", backupPath)
	}

	// Verify backup content
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}

	if !strings.Contains(string(content), "CREATE TABLE test") {
		t.Errorf("Backup content is incorrect")
	}
}

// TestLocalStorageCleanup tests local storage cleanup functionality without database setup
func TestLocalStorageCleanup(t *testing.T) {
	// Create temporary directory for testing
	tempDir := "/tmp/test-cleanup"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create local storage instance
	localStorage, err := storage.NewLocalStorage(&config.LocalConfig{
		Path: tempDir,
	}, logrus.New())
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Create old backup directory (2 days ago)
	oldDate := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	oldBackupDir := filepath.Join(tempDir, "test-backup", "testdb", oldDate)
	if err := os.MkdirAll(oldBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create old backup directory: %v", err)
	}

	// Create old backup file
	oldBackupFile := filepath.Join(oldBackupDir, "testdb_old.sql")
	if err := os.WriteFile(oldBackupFile, []byte("-- Old backup"), 0644); err != nil {
		t.Fatalf("Failed to create old backup file: %v", err)
	}

	// Create new backup directory (today)
	newDate := time.Now().Format("2006-01-02")
	newBackupDir := filepath.Join(tempDir, "test-backup", "testdb", newDate)
	if err := os.MkdirAll(newBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create new backup directory: %v", err)
	}

	// Create new backup file
	newBackupFile := filepath.Join(newBackupDir, "testdb_new.sql")
	if err := os.WriteFile(newBackupFile, []byte("-- New backup"), 0644); err != nil {
		t.Fatalf("Failed to create new backup file: %v", err)
	}

	// Verify both files exist before cleanup
	if _, err := os.Stat(oldBackupFile); os.IsNotExist(err) {
		t.Fatalf("Old backup file should exist before cleanup")
	}
	if _, err := os.Stat(newBackupFile); os.IsNotExist(err) {
		t.Fatalf("New backup file should exist before cleanup")
	}

	// Run cleanup with 1 day retention
	if err := localStorage.DeleteOldBackups("test-backup", 1); err != nil {
		t.Fatalf("Failed to cleanup old backups: %v", err)
	}

	// Verify old backup is deleted
	if _, err := os.Stat(oldBackupFile); !os.IsNotExist(err) {
		t.Errorf("Old backup file should be deleted after cleanup")
	}

	// Verify new backup still exists
	if _, err := os.Stat(newBackupFile); os.IsNotExist(err) {
		t.Errorf("New backup file should still exist after cleanup")
	}
}
