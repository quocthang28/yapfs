package utils

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

// ResolveDestinationPath resolves the destination path, validating directories
func ResolveDestinationPath(destPath string) (string, error) {
	// Check if the path exists and is a directory
	if info, err := os.Stat(destPath); err == nil {
		if info.IsDir() {
			// Valid directory - return as is (filename will come from metadata)
			return destPath, nil
		}
		// Path exists but is not a directory
		return "", fmt.Errorf("destination path '%s' exists but is not a directory", destPath)
	} else if os.IsNotExist(err) {
		// Path doesn't exist - this could be a file path or non-existent directory
		dir := filepath.Dir(destPath)
		if info, dirErr := os.Stat(dir); dirErr == nil && info.IsDir() {
			// Parent exists and is a directory - treat destPath as intended directory name
			// We'll create it when needed
			return destPath, nil
		}
		// Parent doesn't exist
		return "", fmt.Errorf("parent directory does not exist: %s", dir)
	} else {
		// Other error accessing the path
		return "", fmt.Errorf("cannot access destination path: %w", err)
	}
}

// FormatFileSize formats file size in human readable format
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// calculateFileChecksum calculates SHA-256 checksum of a file
func CalculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file for checksum: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func IsFileChecksumMatched(filePath, checksum string) (bool, error) {
	fileChecksum, err := CalculateFileChecksum(filePath)
	if err != nil {
		return false, fmt.Errorf("failed to calculate file checksum: %s", err)
	}

	return checksum == fileChecksum, nil
}

func CreateFileMetadata(filePath string) (types.FileMetadata, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return types.FileMetadata{}, fmt.Errorf("failed to get file info: %w", err)
	}

	// Get filename
	filename := filepath.Base(filePath)

	// Detect MIME type
	mimeType := mime.TypeByExtension(filepath.Ext(filePath))
	if mimeType == "" {
		mimeType = "application/octet-stream" // Default for unknown types
	}

	// Calculate checksum
	checksum, err := CalculateFileChecksum(filePath)
	if err != nil {
		return types.FileMetadata{}, fmt.Errorf("failed to calculate file checksum: %w", err)
	}

	metadata := types.FileMetadata{
		Name:     filename,
		Size:     stat.Size(),
		MimeType: mimeType,
		Checksum: checksum,
	}

	return metadata, nil
}
