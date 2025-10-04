package unit

import (
	"os"
	"path/filepath"
	"testing"

	"db-backuper/internal/config"
	"db-backuper/internal/restore"

	"github.com/sirupsen/logrus"
)

// TestRestoreConfigurationValidation tests restore configuration validation
func TestRestoreConfigurationValidation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name        string
		config      *config.ImportConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid configuration",
			config: &config.ImportConfig{
				TargetDatabase: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Username: "testuser",
					Password: "testpass",
					Database: "testdb",
					SSLMode:  "disable",
				},
				BackupPath:   "/tmp/test_backup.sql",
				DropExisting: true,
			},
			expectError: false,
		},
		{
			name: "Missing target database host",
			config: &config.ImportConfig{
				TargetDatabase: config.DatabaseConfig{
					Port:     5432,
					Username: "testuser",
					Password: "testpass",
					Database: "testdb",
					SSLMode:  "disable",
				},
				BackupPath:   "/tmp/test_backup.sql",
				DropExisting: true,
			},
			expectError: true,
			errorMsg:    "import configuration is incomplete",
		},
		{
			name: "Missing backup path",
			config: &config.ImportConfig{
				TargetDatabase: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Username: "testuser",
					Password: "testpass",
					Database: "testdb",
					SSLMode:  "disable",
				},
				DropExisting: true,
			},
			expectError: true,
			errorMsg:    "import configuration is incomplete",
		},
		{
			name: "Missing target database name",
			config: &config.ImportConfig{
				TargetDatabase: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					Username: "testuser",
					Password: "testpass",
					SSLMode:  "disable",
				},
				BackupPath:   "/tmp/test_backup.sql",
				DropExisting: true,
			},
			expectError: true,
			errorMsg:    "import configuration is incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't test the actual import without a real database,
			// but we can test the configuration validation
			cfg := &config.Config{Import: *tt.config}
			err := cfg.ValidateImportConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestRestoreBackupFileValidation tests backup file validation
func TestRestoreBackupFileValidation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a temporary backup file
	tempDir := "/tmp/test_restore_validation"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tempBackupFile := filepath.Join(tempDir, "test_backup.sql")
	testContent := "-- PostgreSQL database dump\nCREATE TABLE test (id int);\n"

	if err := os.WriteFile(tempBackupFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}

	// Test with existing backup file
	importConfig := &config.ImportConfig{
		TargetDatabase: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Username: "testuser",
			Password: "testpass",
			Database: "testdb",
			SSLMode:  "disable",
		},
		BackupPath:   tempBackupFile,
		DropExisting: true,
	}

	// Test configuration validation (should pass)
	cfg := &config.Config{Import: *importConfig}
	if err := cfg.ValidateImportConfig(); err != nil {
		t.Errorf("Configuration validation failed: %v", err)
	}

	// Test with non-existent backup file
	// Note: The validation only checks if the path is provided, not if the file exists
	// The actual file existence check happens during the import process
	importConfig.BackupPath = "/non/existent/backup.sql"
	cfg = &config.Config{Import: *importConfig}
	if err := cfg.ValidateImportConfig(); err != nil {
		t.Errorf("Configuration validation should pass even with non-existent file: %v", err)
	}
}

// TestImportConfigWithEmptyDatabases tests that import config allows empty databases
func TestImportConfigWithEmptyDatabases(t *testing.T) {
	// Create a temporary config file with empty databases
	tempDir := "/tmp/test_import_empty_db"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "import_config.json")
	configContent := `{
		"databases": [],
		"import": {
			"target_database": {
				"host": "localhost",
				"port": 5432,
				"username": "testuser",
				"password": "testpass",
				"database": "testdb",
				"ssl_mode": "disable"
			},
			"backup_path": "/tmp/test_backup.sql",
			"drop_existing": true
		},
		"logging": {
			"level": "info",
			"format": "text"
		}
	}`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test LoadConfigForImport (should succeed with empty databases)
	cfg, err := config.LoadConfigForImport(configFile)
	if err != nil {
		t.Errorf("LoadConfigForImport should succeed with empty databases: %v", err)
	}
	if cfg == nil {
		t.Error("Expected config, got nil")
	}

	// Test regular LoadConfig (should fail with empty databases)
	_, err = config.LoadConfig(configFile)
	if err == nil {
		t.Error("LoadConfig should fail with empty databases")
	}
	if !contains(err.Error(), "at least one database must be configured") {
		t.Errorf("Expected error about missing databases, got: %v", err)
	}
}

// TestRestoreInstanceCreation tests restore instance creation
func TestRestoreInstanceCreation(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	importConfig := &config.ImportConfig{
		TargetDatabase: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Username: "testuser",
			Password: "testpass",
			Database: "testdb",
			SSLMode:  "disable",
		},
		BackupPath:   "/tmp/test_backup.sql",
		DropExisting: true,
	}

	postgresRestore := restore.NewPostgresImport(importConfig, logger)

	if postgresRestore == nil {
		t.Error("Expected PostgresImport instance, got nil")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

// containsSubstring is a helper function for contains
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
