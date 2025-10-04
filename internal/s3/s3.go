package s3

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"db-backuper/internal/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sirupsen/logrus"
)

// S3Manager handles AWS S3 operations
type S3Manager struct {
	config *config.AWSConfig
	logger *logrus.Logger
	s3     *s3.S3
}

// NewS3Manager creates a new S3 manager instance
func NewS3Manager(awsConfig *config.AWSConfig, logger *logrus.Logger) (*S3Manager, error) {
	// Create AWS session configuration
	awsConfigObj := &aws.Config{
		Region: aws.String(awsConfig.Region),
	}

	// Only set static credentials if they are provided (for local development)
	// In Lambda, we rely on IAM role for authentication
	if awsConfig.AccessKeyID != "" && awsConfig.SecretAccessKey != "" {
		awsConfigObj.Credentials = credentials.NewStaticCredentials(
			awsConfig.AccessKeyID,
			awsConfig.SecretAccessKey,
			"",
		)
	}

	// Create AWS session
	sess, err := session.NewSession(awsConfigObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3Manager{
		config: awsConfig,
		logger: logger,
		s3:     s3.New(sess),
	}, nil
}

// UploadBackup uploads a backup file to S3
func (s *S3Manager) UploadBackup(localFilePath, backupPrefix, databaseName string) (string, error) {
	// Generate S3 key with database-specific path and timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Base(localFilePath)
	s3Key := fmt.Sprintf("%s/%s/%s/%s", backupPrefix, databaseName, timestamp[:10], filename)

	// Open the file
	file, err := os.Open(localFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", localFilePath, err)
	}
	defer file.Close()

	// Create uploader
	uploader := s3manager.NewUploaderWithClient(s.s3)

	// Upload the file
	s.logger.Infof("Uploading backup to S3: s3://%s/%s", s.config.Bucket, s3Key)

	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(s3Key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3: %w", err)
	}

	s.logger.Infof("Backup uploaded successfully to: %s", result.Location)
	return s3Key, nil
}

// DeleteOldBackups deletes backup files older than the specified retention period
func (s *S3Manager) DeleteOldBackups(backupPrefix string, retentionDays int) error {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	s.logger.Infof("Deleting backups older than %d days (before %s)", retentionDays, cutoffDate.Format("2006-01-02"))

	// List objects with the backup prefix
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.Bucket),
		Prefix: aws.String(backupPrefix + "/"),
	}

	var objectsToDelete []*s3.ObjectIdentifier
	err := s.s3.ListObjectsV2Pages(listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			// Parse the date from the S3 key
			// Expected format: backup-prefix/database-name/YYYY-MM-DD/filename
			keyParts := strings.Split(*obj.Key, "/")
			if len(keyParts) >= 3 {
				dateStr := keyParts[2]
				if objDate, err := time.Parse("2006-01-02", dateStr); err == nil {
					if objDate.Before(cutoffDate) {
						objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
							Key: obj.Key,
						})
						s.logger.Infof("Marking for deletion: %s (date: %s)", *obj.Key, dateStr)
					}
				}
			}
		}
		return true
	})

	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objectsToDelete) == 0 {
		s.logger.Info("No old backups found to delete")
		return nil
	}

	// Delete objects in batches
	const maxBatchSize = 1000
	for i := 0; i < len(objectsToDelete); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		batch := objectsToDelete[i:end]
		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.config.Bucket),
			Delete: &s3.Delete{
				Objects: batch,
			},
		}

		result, err := s.s3.DeleteObjects(deleteInput)
		if err != nil {
			return fmt.Errorf("failed to delete objects: %w", err)
		}

		s.logger.Infof("Deleted %d backup files", len(result.Deleted))
		if len(result.Errors) > 0 {
			s.logger.Warnf("Encountered %d errors during deletion", len(result.Errors))
			for _, err := range result.Errors {
				s.logger.Errorf("Failed to delete %s: %s", *err.Key, *err.Message)
			}
		}
	}

	return nil
}

// TestConnection tests the S3 connection
func (s *S3Manager) TestConnection() error {
	_, err := s.s3.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(s.config.Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to access S3 bucket %s: %w", s.config.Bucket, err)
	}

	s.logger.Info("S3 connection test successful")
	return nil
}
