// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package file

import (
	"fmt"
	"os"
	"path/filepath"
)

// fileService implements FileService interface
type fileService struct{}

// NewFileService creates a new file service
func NewFileService() FileService {
	return &fileService{}
}

// OpenReader opens a file for reading and returns file info
func (f *fileService) OpenReader(filePath string) (FileReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &fileReader{
		file: file,
		size: stat.Size(),
		name: stat.Name(),
	}, nil
}

// CreateWriter creates a file for writing
func (f *fileService) CreateWriter(dstPath string) (FileWriter, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}

	return &fileWriter{
		file: file,
		path: dstPath,
	}, nil
}

// GetFileInfo returns information about a file
func (f *fileService) GetFileInfo(filePath string) (FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &fileInfo{
		name: stat.Name(),
		size: stat.Size(),
		path: filePath,
	}, nil
}

// FormatFileSize formats file size in human readable format
func (f *fileService) FormatFileSize(size int64) string {
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

// fileReader implements FileReader interface
type fileReader struct {
	file *os.File
	size int64
	name string
}

func (f *fileReader) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

func (f *fileReader) Close() error {
	return f.file.Close()
}

func (f *fileReader) Size() int64 {
	return f.size
}

func (f *fileReader) Name() string {
	return f.name
}

// fileWriter implements FileWriter interface
type fileWriter struct {
	file *os.File
	path string
}

func (f *fileWriter) Write(p []byte) (n int, err error) {
	return f.file.Write(p)
}

func (f *fileWriter) Close() error {
	return f.file.Close()
}

func (f *fileWriter) Path() string {
	return f.path
}

// fileInfo implements FileInfo interface
type fileInfo struct {
	name string
	size int64
	path string
}

func (f *fileInfo) Name() string {
	return f.name
}

func (f *fileInfo) Size() int64 {
	return f.size
}

func (f *fileInfo) Path() string {
	return f.path
}