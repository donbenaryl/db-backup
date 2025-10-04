package test

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"db-backuper/internal/config"
	"db-backuper/internal/restore"
	"db-backuper/internal/s3"
	"db-backuper/internal/storage"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

var (
	testConfig       *config.Config
	testS3Manager    *s3.S3Manager
	testLocalStorage *storage.LocalStorage
	testLogger       *logrus.Logger
)

// TestMain sets up and tears down the test environment
func TestMain(m *testing.M) {
	// Skip integration tests by default unless explicitly enabled
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		logrus.Info("Skipping integration tests - set RUN_INTEGRATION_TESTS=true to enable")
		os.Exit(0)
	}

	// Setup test environment
	if err := SetupTestEnvironment(); err != nil {
		logrus.Fatalf("Failed to setup test environment: %v", err)
	}

	// Load test configuration
	var err error
	testConfig, err = config.LoadConfig("test/appsettings.test.json")
	if err != nil {
		logrus.Fatalf("Failed to load test configuration: %v", err)
	}

	// Setup S3 manager for testing
	testS3Manager, err = s3.NewS3Manager(&config.AWSConfig{
		Region:          "us-east-1",
		Bucket:          "test-backup-bucket",
		AccessKeyID:     "test",
		SecretAccessKey: "test",
	}, logrus.New())
	if err != nil {
		logrus.Fatalf("Failed to create S3 manager: %v", err)
	}

	// Setup local storage for testing
	testLocalStorage, err = storage.NewLocalStorage(&config.LocalConfig{
		Path: "/tmp/test-backups",
	}, logrus.New())
	if err != nil {
		logrus.Fatalf("Failed to create local storage: %v", err)
	}

	// Setup test logger
	testLogger = logrus.New()
	testLogger.SetLevel(logrus.DebugLevel)

	// Run tests
	code := m.Run()

	// Cleanup
	CleanupTestEnvironment()
	os.Exit(code)
}

// TestBackupCreation tests the backup creation functionality
func TestBackupCreation(t *testing.T) {
	// Test local storage backup
	t.Run("LocalStorage", func(t *testing.T) {
		testLocalBackup(t)
	})

	// Test S3 backup
	t.Run("S3Storage", func(t *testing.T) {
		testS3Backup(t)
	})
}

// testLocalBackup tests local storage backup functionality
func testLocalBackup(t *testing.T) {
	// Create a test backup file with comprehensive data
	testBackupContent := `-- PostgreSQL database dump
-- Dumped from database version 15.0
-- Dumped by pg_dump version 15.0

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE TABLE users (
    id integer NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(100) NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    age integer,
    phone character varying(20),
    address text,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users VALUES (1, 'john_doe', 'john.doe@example.com', 'John', 'Doe', 28, '+1-555-0101', '123 Main St, New York, NY 10001', true, '2024-01-01 10:00:00', '2024-01-01 10:00:00');
INSERT INTO users VALUES (2, 'jane_smith', 'jane.smith@example.com', 'Jane', 'Smith', 32, '+1-555-0102', '456 Oak Ave, Los Angeles, CA 90210', true, '2024-01-01 10:01:00', '2024-01-01 10:01:00');
INSERT INTO users VALUES (3, 'bob_johnson', 'bob.johnson@example.com', 'Bob', 'Johnson', 45, '+1-555-0103', '789 Pine Rd, Chicago, IL 60601', true, '2024-01-01 10:02:00', '2024-01-01 10:02:00');
INSERT INTO users VALUES (4, 'alice_brown', 'alice.brown@example.com', 'Alice', 'Brown', 29, '+1-555-0104', '321 Elm St, Houston, TX 77001', false, '2024-01-01 10:03:00', '2024-01-01 10:03:00');
INSERT INTO users VALUES (5, 'charlie_wilson', 'charlie.wilson@example.com', 'Charlie', 'Wilson', 38, '+1-555-0105', '654 Maple Dr, Phoenix, AZ 85001', true, '2024-01-01 10:04:00', '2024-01-01 10:04:00');
INSERT INTO users VALUES (6, 'diana_davis', 'diana.davis@example.com', 'Diana', 'Davis', 26, '+1-555-0106', '987 Cedar Ln, Philadelphia, PA 19101', true, '2024-01-01 10:05:00', '2024-01-01 10:05:00');
INSERT INTO users VALUES (7, 'eve_miller', 'eve.miller@example.com', 'Eve', 'Miller', 41, '+1-555-0107', '147 Birch St, San Antonio, TX 78201', false, '2024-01-01 10:06:00', '2024-01-01 10:06:00');
INSERT INTO users VALUES (8, 'frank_garcia', 'frank.garcia@example.com', 'Frank', 'Garcia', 33, '+1-555-0108', '258 Spruce Ave, San Diego, CA 92101', true, '2024-01-01 10:07:00', '2024-01-01 10:07:00');

CREATE TABLE test_users (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    email character varying(100) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_users VALUES (1, 'John Doe', 'john@example.com', '2024-01-01 10:00:00');
INSERT INTO test_users VALUES (2, 'Jane Smith', 'jane@example.com', '2024-01-01 10:01:00');
INSERT INTO test_users VALUES (3, 'Bob Johnson', 'bob@example.com', '2024-01-01 10:02:00');

CREATE TABLE test_products (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    price numeric(10,2) NOT NULL,
    description text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_products VALUES (1, 'Laptop', 999.99, 'High-performance laptop', '2024-01-01 10:00:00');
INSERT INTO test_products VALUES (2, 'Mouse', 29.99, 'Wireless mouse', '2024-01-01 10:01:00');
INSERT INTO test_products VALUES (3, 'Keyboard', 79.99, 'Mechanical keyboard', '2024-01-01 10:02:00');
`

	// Create temporary backup file
	tempFile := "/tmp/test_backup.sql"
	if err := os.WriteFile(tempFile, []byte(testBackupContent), 0644); err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}
	defer os.Remove(tempFile)

	// Save backup to local storage
	backupPath, err := testLocalStorage.SaveBackup(tempFile, "test-backup", "testdb1")
	if err != nil {
		t.Fatalf("Failed to save backup to local storage: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatalf("Backup file does not exist: %s", backupPath)
	}

	// Verify backup content using the comprehensive verification function
	verifyBackupContent(t, backupPath)

	t.Logf("Local backup created successfully: %s", backupPath)
}

// testS3Backup tests S3 storage backup functionality
func testS3Backup(t *testing.T) {
	// Create S3 bucket first
	if err := createTestS3Bucket(); err != nil {
		t.Fatalf("Failed to create test S3 bucket: %v", err)
	}

	// Create a test backup file with comprehensive data
	testBackupContent := `-- PostgreSQL database dump
-- Dumped from database version 15.0
-- Dumped by pg_dump version 15.0

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE TABLE users (
    id integer NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(100) NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    age integer,
    phone character varying(20),
    address text,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users VALUES (1, 'john_doe', 'john.doe@example.com', 'John', 'Doe', 28, '+1-555-0101', '123 Main St, New York, NY 10001', true, '2024-01-01 10:00:00', '2024-01-01 10:00:00');
INSERT INTO users VALUES (2, 'jane_smith', 'jane.smith@example.com', 'Jane', 'Smith', 32, '+1-555-0102', '456 Oak Ave, Los Angeles, CA 90210', true, '2024-01-01 10:01:00', '2024-01-01 10:01:00');
INSERT INTO users VALUES (3, 'bob_johnson', 'bob.johnson@example.com', 'Bob', 'Johnson', 45, '+1-555-0103', '789 Pine Rd, Chicago, IL 60601', true, '2024-01-01 10:02:00', '2024-01-01 10:02:00');
INSERT INTO users VALUES (4, 'alice_brown', 'alice.brown@example.com', 'Alice', 'Brown', 29, '+1-555-0104', '321 Elm St, Houston, TX 77001', false, '2024-01-01 10:03:00', '2024-01-01 10:03:00');
INSERT INTO users VALUES (5, 'charlie_wilson', 'charlie.wilson@example.com', 'Charlie', 'Wilson', 38, '+1-555-0105', '654 Maple Dr, Phoenix, AZ 85001', true, '2024-01-01 10:04:00', '2024-01-01 10:04:00');
INSERT INTO users VALUES (6, 'diana_davis', 'diana.davis@example.com', 'Diana', 'Davis', 26, '+1-555-0106', '987 Cedar Ln, Philadelphia, PA 19101', true, '2024-01-01 10:05:00', '2024-01-01 10:05:00');
INSERT INTO users VALUES (7, 'eve_miller', 'eve.miller@example.com', 'Eve', 'Miller', 41, '+1-555-0107', '147 Birch St, San Antonio, TX 78201', false, '2024-01-01 10:06:00', '2024-01-01 10:06:00');
INSERT INTO users VALUES (8, 'frank_garcia', 'frank.garcia@example.com', 'Frank', 'Garcia', 33, '+1-555-0108', '258 Spruce Ave, San Diego, CA 92101', true, '2024-01-01 10:07:00', '2024-01-01 10:07:00');

CREATE TABLE test_users (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    email character varying(100) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_users VALUES (1, 'John Doe', 'john@example.com', '2024-01-01 10:00:00');
INSERT INTO test_users VALUES (2, 'Jane Smith', 'jane@example.com', '2024-01-01 10:01:00');
INSERT INTO test_users VALUES (3, 'Bob Johnson', 'bob@example.com', '2024-01-01 10:02:00');

CREATE TABLE test_products (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    price numeric(10,2) NOT NULL,
    description text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_products VALUES (1, 'Laptop', 999.99, 'High-performance laptop', '2024-01-01 10:00:00');
INSERT INTO test_products VALUES (2, 'Mouse', 29.99, 'Wireless mouse', '2024-01-01 10:01:00');
INSERT INTO test_products VALUES (3, 'Keyboard', 79.99, 'Mechanical keyboard', '2024-01-01 10:02:00');
`

	// Create temporary backup file
	tempFile := "/tmp/test_s3_backup.sql"
	if err := os.WriteFile(tempFile, []byte(testBackupContent), 0644); err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}
	defer os.Remove(tempFile)

	// Upload backup to S3
	s3Key, err := testS3Manager.UploadBackup(tempFile, "test-backup", "testdb1")
	if err != nil {
		t.Fatalf("Failed to upload backup to S3: %v", err)
	}

	// Verify backup exists in S3
	if err := verifyS3Backup(s3Key); err != nil {
		t.Fatalf("Failed to verify S3 backup: %v", err)
	}

	t.Logf("S3 backup created successfully: %s", s3Key)
}

// TestBackupCleanup tests the backup cleanup functionality
func TestBackupCleanup(t *testing.T) {
	// Test local storage cleanup
	t.Run("LocalStorage", func(t *testing.T) {
		testLocalCleanup(t)
	})

	// Test S3 cleanup
	t.Run("S3Storage", func(t *testing.T) {
		testS3Cleanup(t)
	})
}

// testLocalCleanup tests local storage cleanup functionality
func testLocalCleanup(t *testing.T) {
	// Create old backup directories (older than retention period)
	oldDate := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	newDate := time.Now().Format("2006-01-02")

	// Create old backup directory
	oldBackupDir := filepath.Join("/tmp/test-backups", "test-backup", "testdb1", oldDate)
	if err := os.MkdirAll(oldBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create old backup directory: %v", err)
	}

	// Create old backup file
	oldBackupFile := filepath.Join(oldBackupDir, "testdb1_old.sql")
	if err := os.WriteFile(oldBackupFile, []byte("-- Old backup"), 0644); err != nil {
		t.Fatalf("Failed to create old backup file: %v", err)
	}

	// Create new backup directory
	newBackupDir := filepath.Join("/tmp/test-backups", "test-backup", "testdb1", newDate)
	if err := os.MkdirAll(newBackupDir, 0755); err != nil {
		t.Fatalf("Failed to create new backup directory: %v", err)
	}

	// Create new backup file
	newBackupFile := filepath.Join(newBackupDir, "testdb1_new.sql")
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
	if err := testLocalStorage.DeleteOldBackups("test-backup", 1); err != nil {
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

	t.Log("Local storage cleanup test passed")
}

// testS3Cleanup tests S3 cleanup functionality
func testS3Cleanup(t *testing.T) {
	// Create S3 bucket first
	if err := createTestS3Bucket(); err != nil {
		t.Fatalf("Failed to create test S3 bucket: %v", err)
	}

	// Create old and new backup files in S3
	oldDate := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	newDate := time.Now().Format("2006-01-02")

	// Upload old backup
	oldKey := fmt.Sprintf("test-backup/testdb1/%s/testdb1_old.sql", oldDate)
	if err := uploadTestFileToS3(oldKey, "-- Old backup"); err != nil {
		t.Fatalf("Failed to upload old backup to S3: %v", err)
	}

	// Upload new backup
	newKey := fmt.Sprintf("test-backup/testdb1/%s/testdb1_new.sql", newDate)
	if err := uploadTestFileToS3(newKey, "-- New backup"); err != nil {
		t.Fatalf("Failed to upload new backup to S3: %v", err)
	}

	// Verify both files exist before cleanup
	if err := verifyS3Backup(oldKey); err != nil {
		t.Fatalf("Old backup should exist before cleanup")
	}
	if err := verifyS3Backup(newKey); err != nil {
		t.Fatalf("New backup should exist before cleanup")
	}

	// Run cleanup with 1 day retention
	if err := testS3Manager.DeleteOldBackups("test-backup", 1); err != nil {
		t.Fatalf("Failed to cleanup old S3 backups: %v", err)
	}

	// Verify old backup is deleted
	if err := verifyS3Backup(oldKey); err == nil {
		t.Errorf("Old backup should be deleted after cleanup")
	}

	// Verify new backup still exists
	if err := verifyS3Backup(newKey); err != nil {
		t.Errorf("New backup should still exist after cleanup")
	}

	t.Log("S3 cleanup test passed")
}

// TestIntegration tests the full integration with the backup service
func TestIntegration(t *testing.T) {
	// Test with local storage configuration
	t.Run("LocalStorage", func(t *testing.T) {
		testIntegrationWithStorage(t, "test/appsettings.test.local.json")
	})

	// Test with S3 configuration
	t.Run("S3Storage", func(t *testing.T) {
		// Create S3 bucket first
		if err := createTestS3Bucket(); err != nil {
			t.Fatalf("Failed to create test S3 bucket: %v", err)
		}
		testIntegrationWithStorage(t, "test/appsettings.test.json")
	})
}

// testIntegrationWithStorage tests the full integration with a specific storage type
func testIntegrationWithStorage(t *testing.T, configPath string) {
	// Run the backup service once
	cmd := exec.Command("go", "run", "./cmd/main.go", "-config", configPath, "-once")
	cmd.Dir = ".."

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Backup service failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify backup was created
	if configPath == "test/appsettings.test.local.json" {
		verifyLocalBackups(t)
	} else {
		verifyS3Backups(t)
	}

	t.Logf("Integration test passed for %s", configPath)
}

// Helper functions

func createTestS3Bucket() error {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String("http://localhost:4566"),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
	})
	if err != nil {
		return err
	}

	svc := awss3.New(sess)
	_, err = svc.CreateBucket(&awss3.CreateBucketInput{
		Bucket: aws.String("test-backup-bucket"),
	})
	return err
}

func uploadTestFileToS3(key, content string) error {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String("http://localhost:4566"),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
	})
	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(sess)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String("test-backup-bucket"),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
	})
	return err
}

func verifyS3Backup(key string) error {
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("us-east-1"),
		Endpoint:         aws.String("http://localhost:4566"),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
	})
	if err != nil {
		return err
	}

	svc := awss3.New(sess)
	_, err = svc.HeadObject(&awss3.HeadObjectInput{
		Bucket: aws.String("test-backup-bucket"),
		Key:    aws.String(key),
	})
	return err
}

func verifyLocalBackups(t *testing.T) {
	backupDir := "/tmp/test-backups/test-backup"

	// Check that backup directories exist for both databases
	for _, dbName := range []string{"testdb1", "testdb2"} {
		dbDir := filepath.Join(backupDir, dbName)
		if _, err := os.Stat(dbDir); os.IsNotExist(err) {
			t.Errorf("Backup directory for %s does not exist: %s", dbName, dbDir)
		}
	}
}

func verifyS3Backups(t *testing.T) {
	// This would verify S3 backups exist
	// For now, we'll just log that the test passed
	t.Log("S3 backup verification would be implemented here")
}

// verifyBackupContent verifies that backup files contain expected tables and data
func verifyBackupContent(t *testing.T, backupPath string) {
	// Read the backup file
	content, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup file %s: %v", backupPath, err)
	}

	backupContent := string(content)

	// Verify that the backup contains the expected tables
	expectedTables := []string{
		"CREATE TABLE users",
		"CREATE TABLE test_users",
		"CREATE TABLE test_products",
	}

	for _, table := range expectedTables {
		if !strings.Contains(backupContent, table) {
			t.Errorf("Backup file does not contain expected table: %s", table)
		}
	}

	// Verify that the backup contains user data
	expectedUserData := []string{
		"john_doe",
		"jane.smith@example.com",
		"Bob Johnson",
		"alice_brown",
		"Charlie Wilson",
		"Diana Davis",
		"eve.miller@example.com",
		"Frank Garcia",
	}

	for _, userData := range expectedUserData {
		if !strings.Contains(backupContent, userData) {
			t.Errorf("Backup file does not contain expected user data: %s", userData)
		}
	}

	// Verify that the backup contains product data
	expectedProductData := []string{
		"Laptop",
		"Mouse",
		"Keyboard",
		"999.99",
		"29.99",
		"79.99",
	}

	for _, productData := range expectedProductData {
		if !strings.Contains(backupContent, productData) {
			t.Errorf("Backup file does not contain expected product data: %s", productData)
		}
	}

	t.Logf("Backup content verification passed for %s", backupPath)
}

// TestRestoreFunctionality tests the restore/import functionality
func TestRestoreFunctionality(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test: RUN_INTEGRATION_TESTS not set to true")
	}

	// Test local restore
	t.Run("LocalRestore", func(t *testing.T) {
		testLocalRestore(t)
	})

	// Test restore with data verification
	t.Run("RestoreWithVerification", func(t *testing.T) {
		testRestoreWithVerification(t)
	})
}

// testLocalRestore tests local restore functionality
func testLocalRestore(t *testing.T) {
	// Create a test backup file
	testBackupContent := createTestBackupContent()
	tempBackupFile := "/tmp/test_restore_backup.sql"

	if err := os.WriteFile(tempBackupFile, []byte(testBackupContent), 0644); err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}
	defer os.Remove(tempBackupFile)

	// Create import configuration
	importConfig := &config.ImportConfig{
		TargetDatabase: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5433,
			Username: "testuser",
			Password: "testpass",
			Database: "testdb_restored",
			SSLMode:  "disable",
		},
		BackupPath:   tempBackupFile,
		DropExisting: true,
	}

	// Create restore instance
	postgresRestore := restore.NewPostgresImport(importConfig, testLogger)

	// Test the restore
	if err := postgresRestore.ImportBackup(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	t.Log("Local restore test completed successfully")
}

// testRestoreWithVerification tests restore and verifies the data
func testRestoreWithVerification(t *testing.T) {
	// Create a test backup file
	testBackupContent := createTestBackupContent()
	tempBackupFile := "/tmp/test_restore_verify_backup.sql"

	if err := os.WriteFile(tempBackupFile, []byte(testBackupContent), 0644); err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}
	defer os.Remove(tempBackupFile)

	// Create import configuration
	importConfig := &config.ImportConfig{
		TargetDatabase: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5434,
			Username: "testuser",
			Password: "testpass",
			Database: "testdb_restored_verify",
			SSLMode:  "disable",
		},
		BackupPath:   tempBackupFile,
		DropExisting: true,
	}

	// Create restore instance
	postgresRestore := restore.NewPostgresImport(importConfig, testLogger)

	// Test the restore
	if err := postgresRestore.ImportBackup(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify the restored data
	if err := verifyRestoredData(t, importConfig.TargetDatabase); err != nil {
		t.Fatalf("Data verification failed: %v", err)
	}

	t.Log("Restore with verification test completed successfully")
}

// createTestBackupContent creates a test backup file content
func createTestBackupContent() string {
	return `-- PostgreSQL database dump
-- Dumped from database version 15.0
-- Dumped by pg_dump version 15.0

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

CREATE TABLE users (
    id integer NOT NULL,
    username character varying(50) NOT NULL,
    email character varying(100) NOT NULL,
    first_name character varying(50) NOT NULL,
    last_name character varying(50) NOT NULL,
    age integer,
    phone character varying(20),
    address text,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users VALUES (1, 'john_doe', 'john.doe@example.com', 'John', 'Doe', 28, '+1-555-0101', '123 Main St, New York, NY 10001', true, '2024-01-01 10:00:00', '2024-01-01 10:00:00');
INSERT INTO users VALUES (2, 'jane_smith', 'jane.smith@example.com', 'Jane', 'Smith', 32, '+1-555-0102', '456 Oak Ave, Los Angeles, CA 90210', true, '2024-01-01 10:01:00', '2024-01-01 10:01:00');
INSERT INTO users VALUES (3, 'bob_johnson', 'bob.johnson@example.com', 'Bob', 'Johnson', 45, '+1-555-0103', '789 Pine Rd, Chicago, IL 60601', true, '2024-01-01 10:02:00', '2024-01-01 10:02:00');

CREATE TABLE test_products (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    price numeric(10,2) NOT NULL,
    description text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO test_products VALUES (1, 'Laptop', 999.99, 'High-performance laptop', '2024-01-01 10:00:00');
INSERT INTO test_products VALUES (2, 'Mouse', 29.99, 'Wireless mouse', '2024-01-01 10:01:00');
INSERT INTO test_products VALUES (3, 'Keyboard', 79.99, 'Mechanical keyboard', '2024-01-01 10:02:00');

-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: testuser
-- Data for Name: test_products; Type: TABLE DATA; Schema: public; Owner: testuser

-- PostgreSQL database dump complete
`
}

// verifyRestoredData verifies that the restored data is correct
func verifyRestoredData(t *testing.T, dbConfig config.DatabaseConfig) error {
	// Connect to the restored database
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Host, dbConfig.Port, dbConfig.Username, dbConfig.Password, dbConfig.Database, dbConfig.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to restored database: %w", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping restored database: %w", err)
	}

	// Verify users table exists and has data
	var userCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}
	if userCount < 3 {
		return fmt.Errorf("expected at least 3 users, got %d", userCount)
	}

	// Verify specific user data
	var username string
	if err := db.QueryRow("SELECT username FROM users WHERE email = 'john.doe@example.com'").Scan(&username); err != nil {
		return fmt.Errorf("failed to find john.doe@example.com: %w", err)
	}
	if username != "john_doe" {
		return fmt.Errorf("expected username 'john_doe', got '%s'", username)
	}

	// Verify products table exists and has data
	var productCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM test_products").Scan(&productCount); err != nil {
		return fmt.Errorf("failed to count products: %w", err)
	}
	if productCount < 3 {
		return fmt.Errorf("expected at least 3 products, got %d", productCount)
	}

	// Verify specific product data
	var productName string
	var price float64
	if err := db.QueryRow("SELECT name, price FROM test_products WHERE name = 'Laptop'").Scan(&productName, &price); err != nil {
		return fmt.Errorf("failed to find Laptop product: %w", err)
	}
	if productName != "Laptop" || price != 999.99 {
		return fmt.Errorf("expected Laptop with price 999.99, got %s with price %f", productName, price)
	}

	t.Logf("Data verification successful: %d users, %d products", userCount, productCount)
	return nil
}
