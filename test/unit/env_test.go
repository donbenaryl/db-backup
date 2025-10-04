package unit

import (
	"os"
	"path/filepath"
	"testing"

	"db-backuper/internal/config"
)

// TestEnvironmentVariableOverrides tests environment variable override functionality
func TestEnvironmentVariableOverrides(t *testing.T) {
	// Clean up any existing environment variables first
	envVars := []string{
		"DB_HOST", "DB_PORT", "DB_USERNAME", "DB_PASSWORD", "DB_DATABASE", "DB_SSL_MODE",
		"LOCAL_BACKUP_PATH", "BACKUP_RETENTION_DAYS", "BACKUP_SCHEDULE", "BACKUP_PREFIX",
		"LOG_LEVEL", "LOG_FORMAT", "AWS_REGION", "AWS_BUCKET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
	}

	// Store original values and clean up
	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			originalValues[envVar] = val
		}
		os.Unsetenv(envVar)
	}

	// Restore original values after test
	defer func() {
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
			if originalVal, exists := originalValues[envVar]; exists {
				os.Setenv(envVar, originalVal)
			}
		}
	}()

	// Create a temporary config file
	tempDir := "/tmp/test_env_config"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test_config.json")
	configContent := `{
		"databases": [
			{
				"host": "localhost",
				"port": 5432,
				"username": "testuser",
				"password": "testpass",
				"database": "testdb",
				"ssl_mode": "disable"
			}
		],
		"local": {
			"path": "/tmp/backups"
		},
		"backup": {
			"retention_days": 7,
			"schedule": "0 2 * * *",
			"backup_prefix": "test-backup"
		},
		"logging": {
			"level": "info",
			"format": "json"
		}
	}`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Set environment variables to override config (only local storage to avoid validation error)
	os.Setenv("DB_HOST", "env-host")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("DB_USERNAME", "env-user")
	os.Setenv("DB_PASSWORD", "env-pass")
	os.Setenv("DB_DATABASE", "env-db")
	os.Setenv("DB_SSL_MODE", "require")
	os.Setenv("LOCAL_BACKUP_PATH", "/env/backups")
	os.Setenv("BACKUP_RETENTION_DAYS", "14")
	os.Setenv("BACKUP_SCHEDULE", "0 */6 * * *")
	os.Setenv("BACKUP_PREFIX", "env-backup")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_FORMAT", "text")

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_USERNAME")
		os.Unsetenv("DB_PASSWORD")
		os.Unsetenv("DB_DATABASE")
		os.Unsetenv("DB_SSL_MODE")
		os.Unsetenv("LOCAL_BACKUP_PATH")
		os.Unsetenv("BACKUP_RETENTION_DAYS")
		os.Unsetenv("BACKUP_SCHEDULE")
		os.Unsetenv("BACKUP_PREFIX")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_FORMAT")
		// Clean up any AWS environment variables that might be set
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("AWS_BUCKET")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	}()

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify database configuration was overridden
	if len(cfg.Databases) != 1 {
		t.Fatalf("Expected 1 database, got %d", len(cfg.Databases))
	}

	db := cfg.Databases[0]
	if db.Host != "env-host" {
		t.Errorf("Expected host 'env-host', got '%s'", db.Host)
	}
	if db.Port != 5433 {
		t.Errorf("Expected port 5433, got %d", db.Port)
	}
	if db.Username != "env-user" {
		t.Errorf("Expected username 'env-user', got '%s'", db.Username)
	}
	if db.Password != "env-pass" {
		t.Errorf("Expected password 'env-pass', got '%s'", db.Password)
	}
	if db.Database != "env-db" {
		t.Errorf("Expected database 'env-db', got '%s'", db.Database)
	}
	if db.SSLMode != "require" {
		t.Errorf("Expected ssl_mode 'require', got '%s'", db.SSLMode)
	}

	// Verify local storage configuration was overridden
	if cfg.Local.Path != "/env/backups" {
		t.Errorf("Expected local path '/env/backups', got '%s'", cfg.Local.Path)
	}

	// Verify AWS configuration is empty (not configured)
	if cfg.AWS.Region != "" {
		t.Errorf("Expected AWS region to be empty (not configured), got '%s'", cfg.AWS.Region)
	}
	if cfg.AWS.Bucket != "" {
		t.Errorf("Expected AWS bucket to be empty (not configured), got '%s'", cfg.AWS.Bucket)
	}
	if cfg.AWS.AccessKeyID != "" {
		t.Errorf("Expected AWS access key to be empty (not configured), got '%s'", cfg.AWS.AccessKeyID)
	}
	if cfg.AWS.SecretAccessKey != "" {
		t.Errorf("Expected AWS secret key to be empty (not configured), got '%s'", cfg.AWS.SecretAccessKey)
	}

	// Verify backup configuration was overridden
	if cfg.Backup.RetentionDays != 14 {
		t.Errorf("Expected retention days 14, got %d", cfg.Backup.RetentionDays)
	}
	if cfg.Backup.Schedule != "0 */6 * * *" {
		t.Errorf("Expected schedule '0 */6 * * *', got '%s'", cfg.Backup.Schedule)
	}
	if cfg.Backup.BackupPrefix != "env-backup" {
		t.Errorf("Expected backup prefix 'env-backup', got '%s'", cfg.Backup.BackupPrefix)
	}

	// Verify logging configuration was overridden
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("Expected log format 'text', got '%s'", cfg.Logging.Format)
	}
}

// TestDatabaseIndexEnvironmentVariables tests database index-specific environment variables
func TestDatabaseIndexEnvironmentVariables(t *testing.T) {
	// Clean up any existing environment variables first
	envVars := []string{
		"DB_HOST", "DB_PORT", "DB_USERNAME", "DB_PASSWORD", "DB_DATABASE", "DB_SSL_MODE",
		"DB_0_HOST", "DB_0_PORT", "DB_0_USERNAME", "DB_0_PASSWORD", "DB_0_DATABASE", "DB_0_SSL_MODE",
		"DB_1_HOST", "DB_1_PORT", "DB_1_USERNAME", "DB_1_PASSWORD", "DB_1_DATABASE", "DB_1_SSL_MODE",
	}

	// Store original values and clean up
	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			originalValues[envVar] = val
		}
		os.Unsetenv(envVar)
	}

	// Restore original values after test
	defer func() {
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
			if originalVal, exists := originalValues[envVar]; exists {
				os.Setenv(envVar, originalVal)
			}
		}
	}()

	// Create a temporary config file with multiple databases
	tempDir := "/tmp/test_db_index_config"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test_config.json")
	configContent := `{
		"databases": [
			{
				"host": "localhost",
				"port": 5432,
				"username": "user1",
				"password": "pass1",
				"database": "db1",
				"ssl_mode": "disable"
			},
			{
				"host": "localhost",
				"port": 5432,
				"username": "user2",
				"password": "pass2",
				"database": "db2",
				"ssl_mode": "disable"
			}
		],
		"local": {
			"path": "/tmp/backups"
		},
		"backup": {
			"retention_days": 7,
			"schedule": "0 2 * * *",
			"backup_prefix": "test-backup"
		},
		"logging": {
			"level": "info",
			"format": "json"
		}
	}`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Set environment variables to override specific database configurations
	os.Setenv("DB_0_HOST", "env-host-0")
	os.Setenv("DB_0_PORT", "5433")
	os.Setenv("DB_0_USERNAME", "env-user-0")
	os.Setenv("DB_1_HOST", "env-host-1")
	os.Setenv("DB_1_PORT", "5434")
	os.Setenv("DB_1_USERNAME", "env-user-1")

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("DB_0_HOST")
		os.Unsetenv("DB_0_PORT")
		os.Unsetenv("DB_0_USERNAME")
		os.Unsetenv("DB_1_HOST")
		os.Unsetenv("DB_1_PORT")
		os.Unsetenv("DB_1_USERNAME")
	}()

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify database configurations were overridden
	if len(cfg.Databases) != 2 {
		t.Fatalf("Expected 2 databases, got %d", len(cfg.Databases))
	}

	// Check first database
	db0 := cfg.Databases[0]
	if db0.Host != "env-host-0" {
		t.Errorf("Expected DB 0 host 'env-host-0', got '%s'", db0.Host)
	}
	if db0.Port != 5433 {
		t.Errorf("Expected DB 0 port 5433, got %d", db0.Port)
	}
	if db0.Username != "env-user-0" {
		t.Errorf("Expected DB 0 username 'env-user-0', got '%s'", db0.Username)
	}
	// These should remain from config file
	if db0.Password != "pass1" {
		t.Errorf("Expected DB 0 password 'pass1', got '%s'", db0.Password)
	}
	if db0.Database != "db1" {
		t.Errorf("Expected DB 0 database 'db1', got '%s'", db0.Database)
	}

	// Check second database
	db1 := cfg.Databases[1]
	if db1.Host != "env-host-1" {
		t.Errorf("Expected DB 1 host 'env-host-1', got '%s'", db1.Host)
	}
	if db1.Port != 5434 {
		t.Errorf("Expected DB 1 port 5434, got %d", db1.Port)
	}
	if db1.Username != "env-user-1" {
		t.Errorf("Expected DB 1 username 'env-user-1', got '%s'", db1.Username)
	}
	// These should remain from config file
	if db1.Password != "pass2" {
		t.Errorf("Expected DB 1 password 'pass2', got '%s'", db1.Password)
	}
	if db1.Database != "db2" {
		t.Errorf("Expected DB 1 database 'db2', got '%s'", db1.Database)
	}
}

// TestImportEnvironmentVariables tests import-specific environment variables
func TestImportEnvironmentVariables(t *testing.T) {
	// Create a temporary config file for import
	tempDir := "/tmp/test_import_env_config"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test_config.json")
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
			"backup_path": "/tmp/backup.sql",
			"drop_existing": false
		},
		"logging": {
			"level": "info",
			"format": "json"
		}
	}`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Set environment variables to override import configuration
	os.Setenv("IMPORT_DB_HOST", "env-import-host")
	os.Setenv("IMPORT_DB_PORT", "5433")
	os.Setenv("IMPORT_DB_USERNAME", "env-import-user")
	os.Setenv("IMPORT_DB_PASSWORD", "env-import-pass")
	os.Setenv("IMPORT_DB_DATABASE", "env-import-db")
	os.Setenv("IMPORT_DB_SSL_MODE", "require")
	os.Setenv("IMPORT_BACKUP_PATH", "/env/backup.sql")
	os.Setenv("IMPORT_DROP_EXISTING", "true")

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("IMPORT_DB_HOST")
		os.Unsetenv("IMPORT_DB_PORT")
		os.Unsetenv("IMPORT_DB_USERNAME")
		os.Unsetenv("IMPORT_DB_PASSWORD")
		os.Unsetenv("IMPORT_DB_DATABASE")
		os.Unsetenv("IMPORT_DB_SSL_MODE")
		os.Unsetenv("IMPORT_BACKUP_PATH")
		os.Unsetenv("IMPORT_DROP_EXISTING")
	}()

	// Load configuration for import
	cfg, err := config.LoadConfigForImport(configFile)
	if err != nil {
		t.Fatalf("Failed to load import config: %v", err)
	}

	// Verify import database configuration was overridden
	importDB := cfg.Import.TargetDatabase
	if importDB.Host != "env-import-host" {
		t.Errorf("Expected import host 'env-import-host', got '%s'", importDB.Host)
	}
	if importDB.Port != 5433 {
		t.Errorf("Expected import port 5433, got %d", importDB.Port)
	}
	if importDB.Username != "env-import-user" {
		t.Errorf("Expected import username 'env-import-user', got '%s'", importDB.Username)
	}
	if importDB.Password != "env-import-pass" {
		t.Errorf("Expected import password 'env-import-pass', got '%s'", importDB.Password)
	}
	if importDB.Database != "env-import-db" {
		t.Errorf("Expected import database 'env-import-db', got '%s'", importDB.Database)
	}
	if importDB.SSLMode != "require" {
		t.Errorf("Expected import ssl_mode 'require', got '%s'", importDB.SSLMode)
	}

	// Verify import configuration was overridden
	if cfg.Import.BackupPath != "/env/backup.sql" {
		t.Errorf("Expected import backup path '/env/backup.sql', got '%s'", cfg.Import.BackupPath)
	}
	if cfg.Import.DropExisting != true {
		t.Errorf("Expected import drop_existing true, got %v", cfg.Import.DropExisting)
	}
}

// TestEnvironmentVariablePriority tests that environment variables take priority over config file
func TestEnvironmentVariablePriority(t *testing.T) {
	// Clean up any existing environment variables first
	envVars := []string{
		"DB_HOST", "DB_PORT", "DB_USERNAME", "DB_PASSWORD", "DB_DATABASE", "DB_SSL_MODE",
		"LOCAL_BACKUP_PATH", "BACKUP_RETENTION_DAYS", "BACKUP_SCHEDULE", "BACKUP_PREFIX",
		"LOG_LEVEL", "LOG_FORMAT", "AWS_REGION", "AWS_BUCKET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
	}

	// Store original values and clean up
	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			originalValues[envVar] = val
		}
		os.Unsetenv(envVar)
	}

	// Restore original values after test
	defer func() {
		for _, envVar := range envVars {
			os.Unsetenv(envVar)
			if originalVal, exists := originalValues[envVar]; exists {
				os.Setenv(envVar, originalVal)
			}
		}
	}()

	// Create a temporary config file
	tempDir := "/tmp/test_env_priority"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configFile := filepath.Join(tempDir, "test_config.json")
	configContent := `{
		"databases": [
			{
				"host": "config-host",
				"port": 5432,
				"username": "config-user",
				"password": "config-pass",
				"database": "config-db",
				"ssl_mode": "disable"
			}
		],
		"local": {
			"path": "/config/backups"
		},
		"backup": {
			"retention_days": 7,
			"schedule": "0 2 * * *",
			"backup_prefix": "config-backup"
		},
		"logging": {
			"level": "info",
			"format": "json"
		}
	}`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Set only some environment variables
	os.Setenv("DB_HOST", "env-host")
	os.Setenv("DB_USERNAME", "env-user")
	os.Setenv("LOCAL_BACKUP_PATH", "/env/backups")
	os.Setenv("BACKUP_RETENTION_DAYS", "14")

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variables override config file values
	db := cfg.Databases[0]
	if db.Host != "env-host" {
		t.Errorf("Expected host 'env-host' (from env), got '%s'", db.Host)
	}
	if db.Username != "env-user" {
		t.Errorf("Expected username 'env-user' (from env), got '%s'", db.Username)
	}
	if db.Password != "config-pass" {
		t.Errorf("Expected password 'config-pass' (from config), got '%s'", db.Password)
	}
	if db.Database != "config-db" {
		t.Errorf("Expected database 'config-db' (from config), got '%s'", db.Database)
	}

	// Verify local storage
	if cfg.Local.Path != "/env/backups" {
		t.Errorf("Expected local path '/env/backups' (from env), got '%s'", cfg.Local.Path)
	}

	// Verify backup configuration
	if cfg.Backup.RetentionDays != 14 {
		t.Errorf("Expected retention days 14 (from env), got %d", cfg.Backup.RetentionDays)
	}
	if cfg.Backup.Schedule != "0 2 * * *" {
		t.Errorf("Expected schedule '0 2 * * *' (from config), got '%s'", cfg.Backup.Schedule)
	}
}
