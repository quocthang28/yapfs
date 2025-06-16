package processor

import (
	"encoding/json"
	"fmt"
	"mime"
	"os"
	"path/filepath"
)

// FileMetadata contains information about the file being transferred
type FileMetadata struct {
	Name     string `json:"name"`     // Original filename
	Size     int64  `json:"size"`     // File size in bytes
	MimeType string `json:"mimeType"` // MIME type of the file
	// Future fields:
	// Checksum string `json:"checksum"` // SHA-256 checksum
}

// FileService handles basic file operations
type FileService struct{}

// NewFileService creates a new file service
func NewFileService() *FileService {
	return &FileService{}
}

// openReader opens a file for reading
func (f *FileService) openReader(filePath string) (*os.File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// createWriter creates a file for writing
func (f *FileService) createWriter(destPath string) (*os.File, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return file, nil
}

// GetFileInfo returns information about a file by path
func (f *FileService) GetFileInfo(filePath string) (os.FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return stat, nil
}

// ensureDir creates directory if it doesn't exist
func (f *FileService) ensureDir(dirPath string) error {
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// createFileMetadata creates metadata for a file and encode it to send
func (f *FileService) createFileMetadata(filePath string) ([]byte, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get filename
	filename := filepath.Base(filePath)

	// Detect MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream" // Default for unknown types
	}

	metadata := &FileMetadata{
		Name:     filename,
		Size:     stat.Size(),
		MimeType: mimeType,
	}

	encoded, err := f.EncodeMetadata(metadata)
	if err != nil {
		return nil, err
	}

	return encoded, nil
}

// EncodeMetadata encodes file metadata to JSON bytes
func (f *FileService) EncodeMetadata(metadata *FileMetadata) ([]byte, error) {
	return json.Marshal(metadata)
}

// decodeMetadata decodes JSON bytes to file metadata
func (f *FileService) decodeMetadata(data []byte) (*FileMetadata, error) {
	var metadata FileMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}
	return &metadata, nil
}
