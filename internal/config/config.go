package config

import (
	"encoding/json"
	"fmt"
	"os"
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
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

// AWSConfig holds AWS S3 configuration
type AWSConfig struct {
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// LocalConfig holds local storage configuration
type LocalConfig struct {
	Path string `json:"path"`
}

// BackupConfig holds backup-specific configuration
type BackupConfig struct {
	RetentionDays int    `json:"retention_days"`
	Schedule      string `json:"schedule"`
	BackupPrefix  string `json:"backup_prefix"`
}

// ImportConfig holds import/restore configuration
type ImportConfig struct {
	TargetDatabase DatabaseConfig `json:"target_database"`
	BackupPath     string         `json:"backup_path"`
	DropExisting   bool           `json:"drop_existing"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// GetConnectionString returns the PostgreSQL connection string
func (d *DatabaseConfig) GetConnectionString() string {
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

	// Validate configuration for import (allows empty databases)
	if err := config.ValidateForImport(); err != nil {
		return nil, fmt.Errorf("import configuration validation failed: %w", err)
	}

	return &config, nil
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
