package processor

import (
	"fmt"
	"io"
	"log"
	"os"
)

// ReaderService handles file reading and chunking operations
type ReaderService struct {
	fileService *FileService
}

// NewReaderService creates a new reader service
func NewReaderService(fileService *FileService) *ReaderService {
	return &ReaderService{
		fileService: fileService,
	}
}

// DataChunk represents a chunk of file data
type DataChunk struct {
	Data []byte
	EOF  bool
}

// fileReader wraps an open file for sending (internal to ReaderService)
type fileReader struct {
	file     *os.File
	fileInfo os.FileInfo
	filePath string
}

// PrepareFileForReading opens file and validates it's ready for reading
func (r *ReaderService) PrepareFileForReading(filePath string) (*fileReader, error) {
	// Open file for reading
	file, err := r.fileService.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	log.Printf("File prepared for reading: %s, size: %d bytes (%s)",
		filePath, stat.Size(), r.fileService.FormatFileSize(stat.Size()))

	reader := &fileReader{
		file:     file,
		fileInfo: stat,
		filePath: filePath,
	}

	return reader, nil
}

// StartReading reads file chunks and sends them through channels
func (r *ReaderService) StartReading(reader *fileReader, chunkSize int) (<-chan DataChunk, <-chan error) {
	dataCh := make(chan DataChunk, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(errCh)
		defer reader.close()

		// Read and send file chunks
		buffer := make([]byte, chunkSize)
		for {
			n, err := reader.file.Read(buffer)
			if err == io.EOF {
				// Send EOF marker
				dataCh <- DataChunk{Data: nil, EOF: true}
				break
			}
			if err != nil {
				errCh <- fmt.Errorf("failed to read file: %w", err)
				return
			}

			// Send data chunk
			data := make([]byte, n)
			copy(data, buffer[:n])
			dataCh <- DataChunk{Data: data, EOF: false}
		}

		log.Printf("File reading completed: %s", reader.filePath)
	}()

	return dataCh, errCh
}

// GetFileInfo returns the file information for the prepared file
func (r *ReaderService) GetFileInfo(reader *fileReader) os.FileInfo {
	return reader.fileInfo
}

// close closes the internal file reader
func (fr *fileReader) close() error {
	return fr.file.Close()
}