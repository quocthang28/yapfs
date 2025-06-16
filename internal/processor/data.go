package processor

import (
	"fmt"
	"os"
	"time"
)

// ProgressUpdate represents file transfer progress information
type ProgressUpdate struct {
	BytesSent     uint64
	BytesTotal    uint64
	Percentage    float64
	Throughput    float64        // bytes/second
	ElapsedTime   time.Duration
}

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

// CreateFileMetadata creates metadata and encode it (for sending) for a file (delegates to FileService)
func (d *DataProcessor) CreateFileMetadata(filePath string) ([]byte, error) {
	return d.fileService.createFileMetadata(filePath)
}

// DecodeMetadata decodes JSON bytes to file metadata (delegates to FileService)
func (d *DataProcessor) DecodeMetadata(data []byte) (*FileMetadata, error) {
	return d.fileService.decodeMetadata(data)
}

// PrepareFileForSending opens file and validates it's ready for sending (delegates to ReaderService)
func (d *DataProcessor) PrepareFileForSending(filePath string) error {
	// Close any existing file reader
	if d.currentReader != nil {
		d.currentReader.close()
	}

	// Prepare file for reading using ReaderService
	reader, err := d.readerService.prepareFileForReading(filePath)
	if err != nil {
		return err
	}

	d.currentReader = reader
	return nil
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

// StartReadingFileWithProgress reads file chunks with progress reporting
func (d *DataProcessor) StartReadingFileWithProgress(chunkSize int) (<-chan DataChunk, <-chan ProgressUpdate, <-chan error) {
	if d.currentReader == nil {
		return nil, nil, nil
	}

	dataCh, progressCh, errCh := d.readerService.startReadingWithProgress(d.currentReader, chunkSize)

	// Clear the reader after transfer starts (ReaderService handles cleanup)
	d.currentReader = nil

	return dataCh, progressCh, errCh
}

// PrepareFileForReceiving opens a destination file for writing with metadata (delegates to WriterService)
func (d *DataProcessor) PrepareFileForReceiving(destDir string, metadata *FileMetadata) (string, error) {
	// Close any existing file writer
	if d.currentWriter != nil {
		d.currentWriter.close()
	}

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
	return totalBytes, err
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
		if err := d.currentWriter.close(); err != nil {
			errs = append(errs, err)
		}
		d.currentWriter = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing files: %v", errs)
	}
	return nil
}
