// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package file

import (
	"io"
)

// FileService handles file operations for P2P file sharing
type FileService interface {
	// OpenReader opens a file for reading and returns file info
	OpenReader(filePath string) (FileReader, error)
	
	// CreateWriter creates a file for writing
	CreateWriter(dstPath string) (FileWriter, error)
	
	// GetFileInfo returns information about a file
	GetFileInfo(filePath string) (FileInfo, error)
	
	// FormatFileSize formats file size in human readable format
	FormatFileSize(size int64) string
}

// FileReader represents a file opened for reading
type FileReader interface {
	io.Reader
	io.Closer
	
	// Size returns the file size in bytes
	Size() int64
	
	// Name returns the file name
	Name() string
}

// FileWriter represents a file opened for writing
type FileWriter interface {
	io.Writer
	io.Closer
	
	// Path returns the file path
	Path() string
}

// FileInfo contains file metadata
type FileInfo interface {
	// Name returns the file name
	Name() string
	
	// Size returns the file size in bytes
	Size() int64
	
	// Path returns the full file path
	Path() string
}

// TransferProgress represents file transfer progress
type TransferProgress struct {
	BytesTransferred int64
	TotalBytes       int64
	TransferRate     float64 // bytes per second
	Percentage       float64
}

// ProgressReporter defines callbacks for transfer progress updates
type ProgressReporter interface {
	OnProgress(progress TransferProgress)
	OnComplete(totalBytes int64)
	OnError(err error)
}