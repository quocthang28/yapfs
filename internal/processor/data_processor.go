package processor

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataProcessor handles file operations, chunking, and reassembly for P2P file sharing
// Future: Will include checksum validation and advanced data processing
type DataProcessor struct{}

// NewDataProcessor creates a new data processor
func NewDataProcessor() *DataProcessor {
	return &DataProcessor{}
}

// OpenReader opens a file for reading
func (d *DataProcessor) OpenReader(filePath string) (*os.File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// CreateWriter creates a file for writing
func (d *DataProcessor) CreateWriter(destPath string) (*os.File, error) {
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

// GetFileInfo returns information about a file
func (d *DataProcessor) GetFileInfo(filePath string) (os.FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return stat, nil
}

// FormatFileSize formats file size in human readable format //TODO: move to utils pkg
func (d *DataProcessor) FormatFileSize(size int64) string {
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
