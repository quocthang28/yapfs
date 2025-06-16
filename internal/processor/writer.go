package processor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// writerService handles file writing operations
type writerService struct {
	fileService *FileService
}

// newWriterService creates a new writer service
func newWriterService(fileService *FileService) *writerService {
	return &writerService{
		fileService: fileService,
	}
}

// fileWriter wraps an open file for receiving (internal to WriterService)
type fileWriter struct {
	file              *os.File
	destPath          string
	totalBytesWritten uint64
	metadata          *FileMetadata // Metadata of the file being received
}

// prepareFileForWriting opens a destination file for writing with metadata
func (w *writerService) prepareFileForWriting(destDir string, metadata *FileMetadata) (*fileWriter, string, error) {
	// Ensure destination directory exists
	if err := w.fileService.ensureDir(destDir); err != nil {
		return nil, "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create full destination path using original filename from metadata
	destPath := filepath.Join(destDir, metadata.Name)

	// Create destination file
	file, err := w.fileService.createWriter(destPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create destination file: %w", err)
	}

	log.Printf("File prepared for writing: %s (original: %s, size: %d bytes, type: %s)",
		destPath, metadata.Name, metadata.Size, metadata.MimeType)

	writer := &fileWriter{
		file:              file,
		destPath:          destPath,
		totalBytesWritten: 0,
		metadata:          metadata,
	}

	return writer, destPath, nil
}

// writeData writes incoming data to the prepared file
func (w *writerService) writeData(writer *fileWriter, data []byte) error {
	if writer == nil {
		return fmt.Errorf("no file prepared for writing")
	}

	n, err := writer.file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	writer.totalBytesWritten += uint64(n)
	return nil
}

// finishWriting completes the file writing and returns total bytes written
func (w *writerService) finishWriting(writer *fileWriter) (uint64, error) {
	if writer == nil {
		return 0, fmt.Errorf("no file prepared for writing")
	}

	totalBytes := writer.totalBytesWritten
	destPath := writer.destPath

	err := writer.close()

	if err != nil {
		return totalBytes, fmt.Errorf("failed to close file: %w", err)
	}

	log.Printf("File writing completed: %s, %d bytes written", destPath, totalBytes)
	return totalBytes, nil
}

// getTotalBytesWritten returns the total number of bytes written so far
// func (w *writerService) getTotalBytesWritten(writer *fileWriter) uint64 {
// 	if writer == nil {
// 		return 0
// 	}
// 	return writer.totalBytesWritten
// }

// close closes the internal file writer
func (fw *fileWriter) close() error {
	return fw.file.Close()
}
