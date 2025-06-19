package processor

import (
	"fmt"
	"log"
	"os"

	"yapfs/pkg/types"
)

// Data channel should be init and manage data processor internally, app layer doesn't need to know about it
// DataProcessor coordinates file operations, chunking, and reassembly for P2P file sharing
// Now uses composition with specialized services for better separation of concerns
type DataProcessor struct {
	fileService   *FileService
	readerService *readerService
	writerService *writerService

	// Current active reader/writer instances
	currentReader *fileReader
	currentWriter *fileWriter

	// Track file completion status
	fileCompleted bool
}

// NewDataProcessor creates a new data processor with composed services
func NewDataProcessor() *DataProcessor {
	fileService := NewFileService()
	readerService := newReaderService(fileService)
	writerService := newWriterService(fileService)

	return &DataProcessor{
		fileService:   fileService,
		readerService: readerService,
		writerService: writerService,
	}
}

// OpenReader opens a file for reading (delegates to FileService)
func (d *DataProcessor) OpenReader(filePath string) (*os.File, error) {
	return d.fileService.openReader(filePath)
}

// CreateWriter creates a file for writing (delegates to FileService)
func (d *DataProcessor) CreateWriter(destPath string) (*os.File, error) {
	return d.fileService.createWriter(destPath)
}

// PrepareFileForSending opens file and validates it's ready for sending, returns metadata (delegates to ReaderService)
func (d *DataProcessor) PrepareFileForSending(filePath string) (*types.FileMetadata, error) {
	// Close any existing file reader
	if d.currentReader != nil {
		d.currentReader.close()
	}

	// Create metadata first
	metadata, err := d.fileService.CreateMetadata(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata: %w", err)
	}

	// Prepare file for reading using ReaderService
	reader, err := d.readerService.prepareFileForReading(filePath)
	if err != nil {
		return nil, err
	}

	d.currentReader = reader
	return metadata, nil
}

// StartReadingFile reads file chunks and sends them through the data channel (delegates to ReaderService)
func (d *DataProcessor) StartReadingFile(chunkSize int) (<-chan DataChunk, <-chan error) {
	if d.currentReader == nil {
		return nil, nil
	}

	dataCh, errCh := d.readerService.startReading(d.currentReader, chunkSize)

	// Clear the reader after transfer starts (ReaderService handles cleanup)
	d.currentReader = nil

	return dataCh, errCh
}

// GetCurrentFileSize returns the size of the currently prepared file for reading
func (d *DataProcessor) GetCurrentFileSize() int64 {
	if d.currentReader == nil {
		return 0
	}
	return d.currentReader.getFileSize()
}

// PrepareFileForReceiving opens a destination file for writing with metadata (delegates to WriterService)
func (d *DataProcessor) PrepareFileForReceiving(destDir string, metadata *types.FileMetadata) (string, error) {
	// Close any existing file writer
	if d.currentWriter != nil {
		d.currentWriter.close()
	}

	// Reset completion status for new file
	d.fileCompleted = false

	// Prepare file for writing using WriterService
	writer, destPath, err := d.writerService.prepareFileForWriting(destDir, metadata)
	if err != nil {
		return "", err
	}

	d.currentWriter = writer
	return destPath, nil
}

// WriteData writes incoming data to the prepared file (delegates to WriterService)
func (d *DataProcessor) WriteData(data []byte) error {
	return d.writerService.writeData(d.currentWriter, data)
}

// FinishReceiving completes the file reception and returns total bytes written (delegates to WriterService)
func (d *DataProcessor) FinishReceiving() (uint64, error) {
	totalBytes, err := d.writerService.finishWriting(d.currentWriter)
	d.currentWriter = nil

	// Mark file as completed only if no error occurred
	if err == nil {
		d.fileCompleted = true
	}

	return totalBytes, err
}

// ClearPartialFile removes a partially written file and cleans up the current writer
// Only clears if the file is not completed (partial/incomplete)
func (d *DataProcessor) ClearPartialFile() error {
	// If file is completed, don't clear it
	if d.fileCompleted {
		return nil
	}

	// If no current writer, nothing to clear
	if d.currentWriter == nil {
		return nil
	}

	// Get the file path before closing
	filePath := d.currentWriter.destPath

	// Close the file first
	if err := d.currentWriter.close(); err != nil {
		// Continue with cleanup even if close fails
		log.Printf("Warning: failed to close file before cleanup: %v\n", err)
	}

	// Remove the partial file
	if err := os.Remove(filePath); err != nil {
		d.currentWriter = nil
		return fmt.Errorf("failed to remove partial file %s: %w", filePath, err)
	}

	// Clear the current writer and reset completion status
	d.currentWriter = nil
	d.fileCompleted = false

	log.Printf("Partial file removed: %s\n", filePath)
	return nil
}

// Close closes both current file reader and writer
func (d *DataProcessor) Close() error {
	var errs []error

	if d.currentReader != nil {
		if err := d.currentReader.close(); err != nil {
			errs = append(errs, err)
		}
		d.currentReader = nil
	}

	if d.currentWriter != nil {
		if err := d.ClearPartialFile(); err != nil {
			errs = append(errs, err)
		}
		d.currentWriter = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing files: %v", errs)
	}
	return nil
}
