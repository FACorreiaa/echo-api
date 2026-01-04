package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// S3Storage implements Storage using Amazon S3 or S3-compatible services
// TODO: Implement using aws-sdk-go-v2
type S3Storage struct {
	bucket   string
	region   string
	endpoint string
	// client *s3.Client // Uncomment when implementing
}

// NewS3Storage creates a new S3 storage instance
// TODO: Initialize S3 client using aws-sdk-go-v2
func NewS3Storage(cfg *Config) (*S3Storage, error) {
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}
	if cfg.S3Region == "" {
		return nil, fmt.Errorf("S3 region is required")
	}

	// TODO: Initialize S3 client
	// Example with aws-sdk-go-v2:
	//
	// awsCfg, err := config.LoadDefaultConfig(context.Background(),
	//     config.WithRegion(cfg.S3Region),
	//     config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
	//         cfg.S3AccessKeyID,
	//         cfg.S3SecretAccessKey,
	//         "",
	//     )),
	// )
	// if err != nil {
	//     return nil, fmt.Errorf("failed to load AWS config: %w", err)
	// }
	//
	// opts := []func(*s3.Options){}
	// if cfg.S3Endpoint != "" {
	//     opts = append(opts, func(o *s3.Options) {
	//         o.BaseEndpoint = aws.String(cfg.S3Endpoint)
	//         o.UsePathStyle = true // Required for MinIO
	//     })
	// }
	//
	// client := s3.NewFromConfig(awsCfg, opts...)

	return &S3Storage{
		bucket:   cfg.S3Bucket,
		region:   cfg.S3Region,
		endpoint: cfg.S3Endpoint,
	}, nil
}

// Upload stores a file in S3 and returns its metadata
func (s *S3Storage) Upload(ctx context.Context, userID uuid.UUID, filename string, contentType string, r io.Reader) (*FileInfo, error) {
	// TODO: Implement S3 upload
	// Example:
	//
	// fileID := uuid.New()
	// key := fmt.Sprintf("%s/%s/%s", userID.String(), fileID.String(), filename)
	//
	// _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
	//     Bucket:      aws.String(s.bucket),
	//     Key:         aws.String(key),
	//     Body:        r,
	//     ContentType: aws.String(contentType),
	// })
	// if err != nil {
	//     return nil, fmt.Errorf("failed to upload to S3: %w", err)
	// }
	//
	// return &FileInfo{
	//     ID:          fileID,
	//     Name:        filename,
	//     ContentType: contentType,
	//     Path:        key,
	//     CreatedAt:   time.Now(),
	// }, nil

	return nil, fmt.Errorf("S3 storage not implemented - please set STORAGE_TYPE=local or implement S3Storage")
}

// Download retrieves a file from S3 by its ID
func (s *S3Storage) Download(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (io.ReadCloser, *FileInfo, error) {
	// TODO: Implement S3 download
	// Example:
	//
	// info, err := s.GetInfo(ctx, userID, fileID)
	// if err != nil {
	//     return nil, nil, err
	// }
	//
	// result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
	//     Bucket: aws.String(s.bucket),
	//     Key:    aws.String(info.Path),
	// })
	// if err != nil {
	//     return nil, nil, fmt.Errorf("failed to download from S3: %w", err)
	// }
	//
	// return result.Body, info, nil

	return nil, nil, fmt.Errorf("S3 storage not implemented")
}

// Delete removes a file from S3 by its ID
func (s *S3Storage) Delete(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) error {
	// TODO: Implement S3 delete
	return fmt.Errorf("S3 storage not implemented")
}

// List returns all files for a user from S3
func (s *S3Storage) List(ctx context.Context, userID uuid.UUID) ([]*FileInfo, error) {
	// TODO: Implement S3 list
	// This would typically list objects with prefix: userID/
	return nil, fmt.Errorf("S3 storage not implemented")
}

// GetInfo returns metadata for a file without downloading
func (s *S3Storage) GetInfo(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (*FileInfo, error) {
	// TODO: Implement S3 head object
	// Example:
	//
	// key := fmt.Sprintf("%s/%s/", userID.String(), fileID.String())
	// result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
	//     Bucket: aws.String(s.bucket),
	//     Prefix: aws.String(key),
	//     MaxKeys: aws.Int32(1),
	// })
	// ...

	return nil, fmt.Errorf("S3 storage not implemented")
}

// GetReader returns a reader for a file from S3
func (s *S3Storage) GetReader(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (io.ReadCloser, error) {
	reader, _, err := s.Download(ctx, userID, fileID)
	return reader, err
}
