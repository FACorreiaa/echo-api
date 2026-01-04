// Package storage provides file storage abstraction with local and S3 implementations.
package storage

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// FileInfo contains metadata about a stored file
type FileInfo struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	Path        string    `json:"path"` // Internal storage path
	CreatedAt   time.Time `json:"created_at"`
}

// Storage defines the interface for file storage operations
type Storage interface {
	// Upload stores a file and returns its metadata
	Upload(ctx context.Context, userID uuid.UUID, filename string, contentType string, r io.Reader) (*FileInfo, error)

	// Download retrieves a file by its ID
	Download(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (io.ReadCloser, *FileInfo, error)

	// Delete removes a file by its ID
	Delete(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) error

	// List returns all files for a user
	List(ctx context.Context, userID uuid.UUID) ([]*FileInfo, error)

	// GetInfo returns metadata for a file without downloading
	GetInfo(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (*FileInfo, error)

	// GetReader returns a reader for a file (for streaming processing)
	GetReader(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (io.ReadCloser, error)
}

// StorageType identifies the storage backend
type StorageType string

const (
	StorageTypeLocal StorageType = "local"
	StorageTypeS3    StorageType = "s3"
)

// Config holds storage configuration
type Config struct {
	Type StorageType `yaml:"type" env:"STORAGE_TYPE" envDefault:"local"`

	// Local storage config
	LocalPath string `yaml:"local_path" env:"STORAGE_LOCAL_PATH" envDefault:"./uploads"`

	// S3 storage config (prepared for future use)
	S3Bucket          string `yaml:"s3_bucket" env:"STORAGE_S3_BUCKET"`
	S3Region          string `yaml:"s3_region" env:"STORAGE_S3_REGION"`
	S3AccessKeyID     string `yaml:"s3_access_key_id" env:"STORAGE_S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `yaml:"s3_secret_access_key" env:"STORAGE_S3_SECRET_ACCESS_KEY"`
	S3Endpoint        string `yaml:"s3_endpoint" env:"STORAGE_S3_ENDPOINT"` // For S3-compatible services (MinIO, etc.)
}

// New creates a new Storage implementation based on configuration
func New(cfg *Config) (Storage, error) {
	switch cfg.Type {
	case StorageTypeS3:
		return NewS3Storage(cfg)
	case StorageTypeLocal:
		fallthrough
	default:
		return NewLocalStorage(cfg.LocalPath)
	}
}
