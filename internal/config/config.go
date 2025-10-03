package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all configuration for the backup application
type Config struct {
	Database DatabaseConfig `json:"database"`
	AWS      AWSConfig      `json:"aws"`
	Local    LocalConfig    `json:"local"`
	Backup   BackupConfig   `json:"backup"`
	Logging  LoggingConfig  `json:"logging"`
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

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
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

// IsAWSStorage returns true if AWS S3 is configured
func (c *Config) IsAWSStorage() bool {
	return c.AWS.Bucket != "" && c.AWS.Region != "" && c.AWS.AccessKeyID != "" && c.AWS.SecretAccessKey != ""
}
