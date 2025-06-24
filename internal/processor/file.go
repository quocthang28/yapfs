package processor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	"yapfs/pkg/types"
)

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

// calculateFileChecksum calculates SHA-256 checksum of a file
func (f *FileService) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close()

	hash := sha256.New()

	// Copy file contents to hash
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file for checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CreateMetadata creates file metadata struct for a file
func (f *FileService) CreateMetadata(filePath string) (*types.FileMetadata, error) {
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

	// Calculate checksum
	checksum, err := f.calculateFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file checksum: %w", err)
	}

	metadata := &types.FileMetadata{
		Name:     filename,
		Size:     stat.Size(),
		MimeType: mimeType,
		Checksum: checksum,
	}

	return metadata, nil
}
