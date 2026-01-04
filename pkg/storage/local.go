package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LocalStorage implements Storage using the local filesystem
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a new local filesystem storage
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorage{basePath: basePath}, nil
}

// Upload stores a file and returns its metadata
func (s *LocalStorage) Upload(ctx context.Context, userID uuid.UUID, filename string, contentType string, r io.Reader) (*FileInfo, error) {
	fileID := uuid.New()

	// Create user directory
	userDir := filepath.Join(s.basePath, userID.String())
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create user directory: %w", err)
	}

	// Sanitize filename and add UUID prefix for uniqueness
	safeFilename := sanitizeFilename(filename)
	storedFilename := fmt.Sprintf("%s_%s", fileID.String()[:8], safeFilename)
	filePath := filepath.Join(userDir, storedFilename)

	// Create file
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Copy content
	size, err := io.Copy(f, r)
	if err != nil {
		os.Remove(filePath) // Cleanup on error
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	info := &FileInfo{
		ID:          fileID,
		Name:        filename,
		Size:        size,
		ContentType: contentType,
		Path:        storedFilename,
		CreatedAt:   time.Now(),
	}

	// Save metadata
	if err := s.saveMetadata(userID, fileID, info); err != nil {
		os.Remove(filePath) // Cleanup on error
		return nil, err
	}

	return info, nil
}

// Download retrieves a file by its ID
func (s *LocalStorage) Download(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (io.ReadCloser, *FileInfo, error) {
	info, err := s.GetInfo(ctx, userID, fileID)
	if err != nil {
		return nil, nil, err
	}

	filePath := filepath.Join(s.basePath, userID.String(), info.Path)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, info, nil
}

// Delete removes a file by its ID
func (s *LocalStorage) Delete(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) error {
	info, err := s.GetInfo(ctx, userID, fileID)
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.basePath, userID.String(), info.Path)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata
	metaPath := filepath.Join(s.basePath, userID.String(), ".meta", fileID.String()+".json")
	os.Remove(metaPath)

	return nil
}

// List returns all files for a user
func (s *LocalStorage) List(ctx context.Context, userID uuid.UUID) ([]*FileInfo, error) {
	metaDir := filepath.Join(s.basePath, userID.String(), ".meta")
	if _, err := os.Stat(metaDir); os.IsNotExist(err) {
		return []*FileInfo{}, nil
	}

	entries, err := os.ReadDir(metaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list metadata: %w", err)
	}

	files := make([]*FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		fileID := strings.TrimSuffix(entry.Name(), ".json")
		id, err := uuid.Parse(fileID)
		if err != nil {
			continue
		}

		info, err := s.GetInfo(ctx, userID, id)
		if err != nil {
			continue
		}
		files = append(files, info)
	}

	return files, nil
}

// GetInfo returns metadata for a file without downloading
func (s *LocalStorage) GetInfo(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (*FileInfo, error) {
	metaPath := filepath.Join(s.basePath, userID.String(), ".meta", fileID.String()+".json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", fileID)
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var info FileInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &info, nil
}

// GetReader returns a reader for a file (for streaming processing)
func (s *LocalStorage) GetReader(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (io.ReadCloser, error) {
	info, err := s.GetInfo(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(s.basePath, userID.String(), info.Path)
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

// saveMetadata saves file metadata to a JSON file
func (s *LocalStorage) saveMetadata(userID, fileID uuid.UUID, info *FileInfo) error {
	metaDir := filepath.Join(s.basePath, userID.String(), ".meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	metaPath := filepath.Join(metaDir, fileID.String()+".json")
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// sanitizeFilename removes unsafe characters from filenames
func sanitizeFilename(name string) string {
	// Replace path separators and other dangerous characters
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(name)
}
