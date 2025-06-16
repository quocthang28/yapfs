package processor

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
	"yapfs/pkg/utils"
)

// readerService handles file reading and chunking operations
type readerService struct {
	fileService *FileService
}

// newReaderService creates a new reader service
func newReaderService(fileService *FileService) *readerService {
	return &readerService{
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

// prepareFileForReading opens file and validates it's ready for reading
func (r *readerService) prepareFileForReading(filePath string) (*fileReader, error) {
	// Open file for reading
	file, err := r.fileService.openReader(filePath)
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
		filePath, stat.Size(), utils.FormatFileSize(stat.Size()))

	reader := &fileReader{
		file:     file,
		fileInfo: stat,
		filePath: filePath,
	}

	return reader, nil
}

// startReading reads file chunks and sends them through channels
func (r *readerService) startReading(reader *fileReader, chunkSize int) (<-chan DataChunk, <-chan error) {
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

// startReadingWithProgress reads file chunks with progress reporting
func (r *readerService) startReadingWithProgress(reader *fileReader, chunkSize int) (<-chan DataChunk, <-chan ProgressUpdate, <-chan error) {
	dataCh := make(chan DataChunk, 1)
	progressCh := make(chan ProgressUpdate, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(dataCh)
		defer close(progressCh)
		defer close(errCh)
		defer reader.close()

		var bytesSent uint64
		totalBytes := uint64(reader.fileInfo.Size())
		startTime := time.Now()
		lastProgressTime := startTime

		// Send initial progress
		progressCh <- ProgressUpdate{
			BytesSent:   0,
			BytesTotal:  totalBytes,
			Percentage:  0.0,
			Throughput:  0.0,
			ElapsedTime: 0,
		}

		// Read and send file chunks
		buffer := make([]byte, chunkSize)
		for {
			n, err := reader.file.Read(buffer)
			if err == io.EOF {
				// Send EOF marker
				dataCh <- DataChunk{Data: nil, EOF: true}

				// Send final progress
				elapsed := time.Since(startTime)
				avgThroughput := float64(bytesSent) / elapsed.Seconds() / (1024 * 1024) // MB/s
				progressCh <- ProgressUpdate{
					BytesSent:   bytesSent,
					BytesTotal:  totalBytes,
					Percentage:  100.0,
					Throughput:  avgThroughput,
					ElapsedTime: elapsed,
				}
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

			bytesSent += uint64(n)

			// Send progress update every second or when significant progress is made
			now := time.Now()
			if now.Sub(lastProgressTime) >= time.Second || bytesSent == totalBytes {
				elapsed := now.Sub(startTime)
				percentage := float64(bytesSent) / float64(totalBytes) * 100.0
				throughput := float64(bytesSent) / elapsed.Seconds() / (1024 * 1024) // MB/s

				progressCh <- ProgressUpdate{
					BytesSent:   bytesSent,
					BytesTotal:  totalBytes,
					Percentage:  percentage,
					Throughput:  throughput,
					ElapsedTime: elapsed,
				}

				lastProgressTime = now
			}
		}

		log.Printf("File reading completed: %s", reader.filePath)
	}()

	return dataCh, progressCh, errCh
}

// close closes the internal file reader
func (fr *fileReader) close() error {
	return fr.file.Close()
}
