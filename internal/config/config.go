package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
)

// Config holds all configuration for the backup application
type Config struct {
	Databases []DatabaseConfig `json:"databases"`
	AWS       AWSConfig        `json:"aws"`
	Local     LocalConfig      `json:"local"`
	Backup    BackupConfig     `json:"backup"`
	Import    ImportConfig     `json:"import"`
	Logging   LoggingConfig    `json:"logging"`
}

// DatabaseConfig holds PostgreSQL connection configuration
type DatabaseConfig struct {
	Host     string `json:"host" env:"DB_HOST"`
	Port     int    `json:"port" env:"DB_PORT"`
	Username string `json:"username" env:"DB_USERNAME"`
	Password string `json:"password" env:"DB_PASSWORD"`
	Database string `json:"database" env:"DB_DATABASE"`
	SSLMode  string `json:"ssl_mode" env:"DB_SSL_MODE"`
}

// AWSConfig holds AWS S3 configuration
type AWSConfig struct {
	Region          string `json:"region" env:"AWS_REGION"`
	Bucket          string `json:"bucket" env:"AWS_BUCKET"`
	AccessKeyID     string `json:"access_key_id" env:"AWS_ACCESS_KEY_ID"`
	SecretAccessKey string `json:"secret_access_key" env:"AWS_SECRET_ACCESS_KEY"`
}

// LocalConfig holds local storage configuration
type LocalConfig struct {
	Path string `json:"path" env:"LOCAL_BACKUP_PATH"`
}

// BackupConfig holds backup-specific configuration
type BackupConfig struct {
	RetentionDays int    `json:"retention_days" env:"BACKUP_RETENTION_DAYS"`
	Schedule      string `json:"schedule" env:"BACKUP_SCHEDULE"`
	BackupPrefix  string `json:"backup_prefix" env:"BACKUP_PREFIX"`
}

// ImportConfig holds import/restore configuration
type ImportConfig struct {
	TargetDatabase ImportDatabaseConfig `json:"target_database"`
	BackupPath     string               `json:"backup_path" env:"IMPORT_BACKUP_PATH"`
	DropExisting   bool                 `json:"drop_existing" env:"IMPORT_DROP_EXISTING"`
}

// ImportDatabaseConfig holds target database configuration for imports
type ImportDatabaseConfig struct {
	Host     string `json:"host" env:"IMPORT_DB_HOST"`
	Port     int    `json:"port" env:"IMPORT_DB_PORT"`
	Username string `json:"username" env:"IMPORT_DB_USERNAME"`
	Password string `json:"password" env:"IMPORT_DB_PASSWORD"`
	Database string `json:"database" env:"IMPORT_DB_DATABASE"`
	SSLMode  string `json:"ssl_mode" env:"IMPORT_DB_SSL_MODE"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `json:"level" env:"LOG_LEVEL"`
	Format string `json:"format" env:"LOG_FORMAT"`
}

// GetConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.Username, d.Password, d.Database, d.SSLMode)
}

// GetConnectionString returns the PostgreSQL connection string for import database
func (d *ImportDatabaseConfig) GetConnectionString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.Username, d.Password, d.Database, d.SSLMode)
}

// LoadConfig loads configuration from appsettings.json
func LoadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Apply environment variable overrides
	if err := applyEnvOverrides(&config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// LoadConfigForImport loads configuration from a JSON file for import operations
func LoadConfigForImport(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Apply environment variable overrides
	if err := applyEnvOverrides(&config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration for import (allows empty databases)
	if err := config.ValidateForImport(); err != nil {
		return nil, fmt.Errorf("import configuration validation failed: %w", err)
	}

	return &config, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func applyEnvOverrides(config *Config) error {
	// Handle database arrays - check for both DB_* and DB_INDEX_* environment variables
	// This allows overriding specific database configurations
	for i := range config.Databases {
		// First try DB_* variables (for the first database)
		if i == 0 {
			if err := parseDatabaseEnv(&config.Databases[i], "DB_"); err != nil {
				return fmt.Errorf("failed to parse database %d environment variables: %w", i, err)
			}
		}

		// Then try DB_INDEX_* variables (for all databases)
		dbPrefix := fmt.Sprintf("DB_%d_", i)
		if err := parseDatabaseEnv(&config.Databases[i], dbPrefix); err != nil {
			return fmt.Errorf("failed to parse database %d environment variables: %w", i, err)
		}
	}

	// Parse environment variables for the main config (excluding databases)
	// We need to parse each section separately to avoid conflicts
	if err := parseConfigSections(config); err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return nil
}

// parseConfigSections parses environment variables for different config sections
func parseConfigSections(config *Config) error {
	// Parse AWS config
	if err := env.Parse(&config.AWS); err != nil {
		return fmt.Errorf("failed to parse AWS environment variables: %w", err)
	}

	// Parse Local config
	if err := env.Parse(&config.Local); err != nil {
		return fmt.Errorf("failed to parse Local environment variables: %w", err)
	}

	// Parse Backup config
	if err := env.Parse(&config.Backup); err != nil {
		return fmt.Errorf("failed to parse Backup environment variables: %w", err)
	}

	// Parse Import config
	if err := env.Parse(&config.Import); err != nil {
		return fmt.Errorf("failed to parse Import environment variables: %w", err)
	}

	// Parse Logging config
	if err := env.Parse(&config.Logging); err != nil {
		return fmt.Errorf("failed to parse Logging environment variables: %w", err)
	}

	return nil
}

// parseDatabaseEnv parses environment variables for a specific database
func parseDatabaseEnv(db *DatabaseConfig, prefix string) error {
	// Create a temporary struct with prefixed env tags
	type TempDB struct {
		Host     string `env:"HOST"`
		Port     int    `env:"PORT"`
		Username string `env:"USERNAME"`
		Password string `env:"PASSWORD"`
		Database string `env:"DATABASE"`
		SSLMode  string `env:"SSL_MODE"`
	}

	tempDB := TempDB{
		Host:     db.Host,
		Port:     db.Port,
		Username: db.Username,
		Password: db.Password,
		Database: db.Database,
		SSLMode:  db.SSLMode,
	}

	// Parse with custom prefix
	opts := env.Options{
		Prefix: prefix,
	}
	if err := env.ParseWithOptions(&tempDB, opts); err != nil {
		return err
	}

	// Update the original database config if environment variables were set
	if os.Getenv(prefix+"HOST") != "" {
		db.Host = tempDB.Host
	}
	if os.Getenv(prefix+"PORT") != "" {
		db.Port = tempDB.Port
	}
	if os.Getenv(prefix+"USERNAME") != "" {
		db.Username = tempDB.Username
	}
	if os.Getenv(prefix+"PASSWORD") != "" {
		db.Password = tempDB.Password
	}
	if os.Getenv(prefix+"DATABASE") != "" {
		db.Database = tempDB.Database
	}
	if os.Getenv(prefix+"SSL_MODE") != "" {
		db.SSLMode = tempDB.SSLMode
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	return c.ValidateForBackup()
}

// ValidateForBackup checks if the configuration is valid for backup operations
func (c *Config) ValidateForBackup() error {
	// Check if databases are configured
	if len(c.Databases) == 0 {
		return fmt.Errorf("at least one database must be configured")
	}

	// Validate each database configuration
	for i, db := range c.Databases {
		if db.Database == "" {
			return fmt.Errorf("database name is required for database %d", i)
		}
		if db.Host == "" {
			return fmt.Errorf("database host is required for database %d", i)
		}
		if db.Username == "" {
			return fmt.Errorf("database username is required for database %d", i)
		}
		if db.Password == "" {
			return fmt.Errorf("database password is required for database %d", i)
		}
	}

	// Check if either local path or AWS S3 is configured
	hasLocal := c.Local.Path != ""
	hasAWS := c.AWS.Bucket != "" && c.AWS.Region != "" && c.AWS.AccessKeyID != "" && c.AWS.SecretAccessKey != ""

	if !hasLocal && !hasAWS {
		return fmt.Errorf("either local storage path or AWS S3 configuration is required")
	}

	if hasLocal && hasAWS {
		return fmt.Errorf("both local storage and AWS S3 are configured, please choose one")
	}

	return nil
}

// IsLocalStorage returns true if local storage is configured
func (c *Config) IsLocalStorage() bool {
	return c.Local.Path != ""
}

// IsImportConfigured returns true if import configuration is valid
func (c *Config) IsImportConfigured() bool {
	return c.Import.BackupPath != "" &&
		c.Import.TargetDatabase.Host != "" &&
		c.Import.TargetDatabase.Database != "" &&
		c.Import.TargetDatabase.Username != "" &&
		c.Import.TargetDatabase.Password != ""
}

// ValidateImportConfig validates the import configuration
func (c *Config) ValidateImportConfig() error {
	if !c.IsImportConfigured() {
		return fmt.Errorf("import configuration is incomplete - requires target_database and backup_path")
	}

	if c.Import.TargetDatabase.Host == "" {
		return fmt.Errorf("import target database host is required")
	}
	if c.Import.TargetDatabase.Database == "" {
		return fmt.Errorf("import target database name is required")
	}
	if c.Import.TargetDatabase.Username == "" {
		return fmt.Errorf("import target database username is required")
	}
	if c.Import.TargetDatabase.Password == "" {
		return fmt.Errorf("import target database password is required")
	}
	if c.Import.BackupPath == "" {
		return fmt.Errorf("import backup path is required")
	}

	return nil
}

// ValidateForImport validates the configuration for import operations (allows empty databases)
func (c *Config) ValidateForImport() error {
	// For import operations, we only need to validate the import configuration
	// Databases array can be empty since we're not backing up anything
	return c.ValidateImportConfig()
}

// IsAWSStorage returns true if AWS S3 is configured
func (c *Config) IsAWSStorage() bool {
	return c.AWS.Bucket != "" && c.AWS.Region != "" && c.AWS.AccessKeyID != "" && c.AWS.SecretAccessKey != ""
}
