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

	return &config, nil
}
