package processor

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// DataProcessor handles file operations, chunking, and reassembly for P2P file sharing
// Future: Will include checksum validation and advanced data processing
type DataProcessor struct {
	fileReader *fileReader
	fileWriter *fileWriter
}

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

// GetFileInfoByPath returns information about a file by path
func (d *DataProcessor) GetFileInfoByPath(filePath string) (os.FileInfo, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return stat, nil
}

// DataChunk represents a chunk of file data
type DataChunk struct {
	Data []byte
	EOF  bool
}

// fileReader wraps an open file for sending (internal to DataProcessor)
type fileReader struct {
	file     *os.File
	fileInfo os.FileInfo
	filePath string
}

// fileWriter wraps an open file for receiving (internal to DataProcessor)
type fileWriter struct {
	file              *os.File
	destPath          string
	totalBytesWritten uint64
}

// PrepareFileForSending opens file and validates it's ready for sending
func (d *DataProcessor) PrepareFileForSending(filePath string) error {
	// Close any existing file reader
	if d.fileReader != nil {
		d.fileReader.close()
	}

	// Open file for reading
	file, err := d.OpenReader(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to get file info: %w", err)
	}

	log.Printf("File prepared for sending: %s, size: %d bytes (%s)", 
		filePath, stat.Size(), d.FormatFileSize(stat.Size()))

	d.fileReader = &fileReader{
		file:     file,
		fileInfo: stat,
		filePath: filePath,
	}

	return nil
}

// StartFileTransfer reads file chunks and sends them through the data channel
func (d *DataProcessor) StartFileTransfer(chunkSize int) (<-chan DataChunk, <-chan error) {
	if d.fileReader == nil {
		return nil, nil
	}

	dataCh := make(chan DataChunk, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(errCh)
		defer d.fileReader.close()

		// Read and send file chunks
		buffer := make([]byte, chunkSize)
		for {
			n, err := d.fileReader.file.Read(buffer)
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

		log.Printf("File transfer completed: %s", d.fileReader.filePath)
		d.fileReader = nil // Clear the reader after transfer
	}()

	return dataCh, errCh
}

// GetFileInfo returns the file information for the prepared file
func (d *DataProcessor) GetFileInfo() (os.FileInfo, error) {
	if d.fileReader == nil {
		return nil, fmt.Errorf("no file prepared")
	}
	return d.fileReader.fileInfo, nil
}

// PrepareFileForReceiving opens a destination file for writing
func (d *DataProcessor) PrepareFileForReceiving(destPath string) error {
	// Close any existing file writer
	if d.fileWriter != nil {
		d.fileWriter.close()
	}

	// Create destination file
	file, err := d.CreateWriter(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	log.Printf("File prepared for receiving: %s", destPath)

	d.fileWriter = &fileWriter{
		file:              file,
		destPath:          destPath,
		totalBytesWritten: 0,
	}

	return nil
}

// WriteData writes incoming data to the prepared file
func (d *DataProcessor) WriteData(data []byte) error {
	if d.fileWriter == nil {
		return fmt.Errorf("no file prepared for writing")
	}

	n, err := d.fileWriter.file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	d.fileWriter.totalBytesWritten += uint64(n)
	return nil
}

// FinishReceiving completes the file reception and returns total bytes written
func (d *DataProcessor) FinishReceiving() (uint64, error) {
	if d.fileWriter == nil {
		return 0, fmt.Errorf("no file prepared for writing")
	}

	totalBytes := d.fileWriter.totalBytesWritten
	destPath := d.fileWriter.destPath

	err := d.fileWriter.close()
	d.fileWriter = nil

	if err != nil {
		return totalBytes, fmt.Errorf("failed to close file: %w", err)
	}

	log.Printf("File reception completed: %s, %d bytes written", destPath, totalBytes)
	return totalBytes, nil
}

// Close closes both current file reader and writer
func (d *DataProcessor) Close() error {
	var errs []error

	if d.fileReader != nil {
		if err := d.fileReader.close(); err != nil {
			errs = append(errs, err)
		}
		d.fileReader = nil
	}

	if d.fileWriter != nil {
		if err := d.fileWriter.close(); err != nil {
			errs = append(errs, err)
		}
		d.fileWriter = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing files: %v", errs)
	}
	return nil
}

// close closes the internal file reader
func (fr *fileReader) close() error {
	return fr.file.Close()
}

// close closes the internal file writer
func (fw *fileWriter) close() error {
	return fw.file.Close()
}

// FormatFileSize formats file size in human readable format // TODO: move to utils pkg
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
