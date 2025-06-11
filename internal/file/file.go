package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileService handles file operations for P2P file sharing
type FileService struct{}

// NewFileService creates a new file service
func NewFileService() *FileService {
	return &FileService{}
}

// OpenReader opens a file for reading and returns file info
func (f *FileService) OpenReader(filePath string) (*FileReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &FileReader{
		file: file,
		size: stat.Size(),
		name: stat.Name(),
	}, nil
}

// CreateWriter creates a file for writing
func (f *FileService) CreateWriter(dstPath string) (*FileWriter, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &FileWriter{
		file: file,
		path: dstPath,
	}, nil
}

// GetFileInfo returns information about a file
func (f *FileService) GetFileInfo(filePath string) (*FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &FileInfo{
		name: stat.Name(),
		size: stat.Size(),
		path: filePath,
	}, nil
}

// FormatFileSize formats file size in human readable format
func (f *FileService) FormatFileSize(size int64) string {
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

// FileReader represents a file opened for reading
type FileReader struct {
	file *os.File
	size int64
	name string
}

func (f *FileReader) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

func (f *FileReader) Close() error {
	return f.file.Close()
}

func (f *FileReader) Size() int64 {
	return f.size
}

func (f *FileReader) Name() string {
	return f.name
}

// FileWriter represents a file opened for writing
type FileWriter struct {
	file *os.File
	path string
}

func (f *FileWriter) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

func (f *FileWriter) Close() error {
	return f.file.Close()
}

func (f *FileWriter) Path() string {
	return f.path
}

// FileInfo contains file metadata
type FileInfo struct {
	name string
	size int64
	path string
}

func (f *FileInfo) Name() string {
	return f.name
}

func (f *FileInfo) Size() int64 {
	return f.size
}

func (f *FileInfo) Path() string {
	return f.path
}
