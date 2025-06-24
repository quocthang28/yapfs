package processor

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
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
	file      *os.File
	fileInfo  os.FileInfo
	filePath  string
	bufReader *bufio.Reader
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
		file:      file,
		fileInfo:  stat,
		filePath:  filePath,
		bufReader: bufio.NewReaderSize(file, 256*1024), // 256KB buffer for optimal I/O
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
			n, err := reader.bufReader.Read(buffer)
			if err == io.EOF {
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
	}()

	return dataCh, errCh
}

// close closes the internal file reader
func (fr *fileReader) close() error {
	return fr.file.Close()
}
