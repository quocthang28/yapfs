package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ChunkType represents the type of chunk being sent
type ChunkType string

const (
	TypeMetadata ChunkType = "metadata"
	TypeData     ChunkType = "data"
	TypeEOF      ChunkType = "EOF"
)

// Chunk represents a data chunk with metadata
type Chunk struct {
	Type        ChunkType `json:"type"`
	Data        []byte    `json:"data"`
	ChunkNumber int       `json:"chunk_number"`
	Checksum    string    `json:"checksum"`
	Timestamp   int64     `json:"timestamp"`
}

// FileMetadata contains information about the file being processed
type FileMetadata struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ModTime     int64  `json:"mod_time"`
	TotalChunks int    `json:"total_chunks"`
	FileHash    string `json:"file_hash"`
}

// ChunkState tracks the state of chunk reception
type ChunkState struct {
	Received  bool
	Chunk     *Chunk
	Timestamp int64
}

// DataProcessor handles file processing and chunking (sender side)
type DataProcessor struct {
	BufferSize  int
	ChunkNumber int
}

// FileReceiver handles receiving and reassembling chunks (receiver side)
type FileReceiver struct {
	mutex           sync.RWMutex
	metadata        *FileMetadata
	chunks          map[int]*ChunkState
	receivedChunks  int
	tempDir         string
	outputPath      string
	isComplete      bool
	expectedChunks  int
}

// NewDataProcessor creates a new data processor with specified buffer size
func NewDataProcessor(bufferSize int) *DataProcessor {
	return &DataProcessor{
		BufferSize:  bufferSize,
		ChunkNumber: 0,
	}
}

// NewFileReceiver creates a new file receiver
func NewFileReceiver(tempDir, outputPath string) *FileReceiver {
	return &FileReceiver{
		chunks:     make(map[int]*ChunkState),
		tempDir:    tempDir,
		outputPath: outputPath,
	}
}

// calculateChecksum generates MD5 checksum for data
func calculateChecksum(data []byte) string {
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

// calculateFileHash generates SHA-256 hash for entire file
func calculateFileHash(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// sendChunkOverNetwork mocks sending chunk over the internet
func (dp *DataProcessor) sendChunkOverNetwork(chunk *Chunk, receiver *FileReceiver) error {
	// Mock network transmission with some delay
	time.Sleep(50 * time.Millisecond)
	
	fmt.Printf("📡 Sending chunk #%d [%s] - Size: %d bytes, Checksum: %s\n", 
		chunk.ChunkNumber, chunk.Type, len(chunk.Data), chunk.Checksum)
	
	// Simulate receiving the chunk on receiver side
	return receiver.ReceiveChunk(chunk)
}

// createMetadataChunk creates a metadata chunk with file information
func (dp *DataProcessor) createMetadataChunk(fileInfo os.FileInfo, totalChunks int, fileHash string) *Chunk {
	dp.ChunkNumber++
	
	metadata := FileMetadata{
		Filename:    fileInfo.Name(),
		Size:        fileInfo.Size(),
		ModTime:     fileInfo.ModTime().Unix(),
		TotalChunks: totalChunks,
		FileHash:    fileHash,
	}
	
	metadataBytes, _ := json.Marshal(metadata)
	
	return &Chunk{
		Type:        TypeMetadata,
		Data:        metadataBytes,
		ChunkNumber: dp.ChunkNumber,
		Checksum:    calculateChecksum(metadataBytes),
		Timestamp:   time.Now().Unix(),
	}
}

// createDataChunk creates a data chunk from buffer
func (dp *DataProcessor) createDataChunk(data []byte) *Chunk {
	dp.ChunkNumber++
	
	return &Chunk{
		Type:        TypeData,
		Data:        data,
		ChunkNumber: dp.ChunkNumber,
		Checksum:    calculateChecksum(data),
		Timestamp:   time.Now().Unix(),
	}
}

// createEOFChunk creates an EOF signal chunk
func (dp *DataProcessor) createEOFChunk() *Chunk {
	dp.ChunkNumber++
	
	eofData := []byte("EOF")
	
	return &Chunk{
		Type:        TypeEOF,
		Data:        eofData,
		ChunkNumber: dp.ChunkNumber,
		Checksum:    calculateChecksum(eofData),
		Timestamp:   time.Now().Unix(),
	}
}

// calculateTotalChunks calculates how many chunks the file will be split into
func (dp *DataProcessor) calculateTotalChunks(fileSize int64) int {
	totalChunks := int(fileSize / int64(dp.BufferSize))
	if fileSize%int64(dp.BufferSize) != 0 {
		totalChunks++
	}
	return totalChunks + 2 // +1 for metadata, +1 for EOF
}

// ProcessFile processes a file by reading it in chunks and sending over network
func (dp *DataProcessor) ProcessFile(filename string, receiver *FileReceiver) error {
	// Open file
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	// Calculate file hash
	fileHash, err := calculateFileHash(filename)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}
	
	// Calculate total chunks
	totalChunks := dp.calculateTotalChunks(fileInfo.Size())
	
	fmt.Printf("🚀 Starting to process file: %s (Size: %d bytes)\n", filename, fileInfo.Size())
	fmt.Printf("📦 Buffer size: %d bytes, Total chunks: %d\n", dp.BufferSize, totalChunks)
	fmt.Printf("🔐 File hash: %s\n\n", fileHash)
	
	// Send metadata chunk first
	metadataChunk := dp.createMetadataChunk(fileInfo, totalChunks, fileHash)
	if err := dp.sendChunkOverNetwork(metadataChunk, receiver); err != nil {
		return fmt.Errorf("failed to send metadata chunk: %w", err)
	}
	
	// Create buffer for reading file
	buffer := make([]byte, dp.BufferSize)
	
	// Process file in chunks
	for {
		// Read data into buffer
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file: %w", err)
		}
		
		// If we read some data, create and send chunk
		if bytesRead > 0 {
			// Create a slice with only the read data
			chunkData := make([]byte, bytesRead)
			copy(chunkData, buffer[:bytesRead])
			
			// Create and send data chunk
			dataChunk := dp.createDataChunk(chunkData)
			if err := dp.sendChunkOverNetwork(dataChunk, receiver); err != nil {
				return fmt.Errorf("failed to send data chunk: %w", err)
			}
		}
		
		// If we reached EOF, break the loop
		if err == io.EOF {
			break
		}
	}
	
	// Send EOF chunk
	eofChunk := dp.createEOFChunk()
	if err := dp.sendChunkOverNetwork(eofChunk, receiver); err != nil {
		return fmt.Errorf("failed to send EOF chunk: %w", err)
	}
	
	fmt.Printf("\n✅ File processing completed! Total chunks sent: %d\n", dp.ChunkNumber)
	return nil
}

// ReceiveChunk handles receiving a chunk from the network
func (fr *FileReceiver) ReceiveChunk(chunk *Chunk) error {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()
	
	fmt.Printf("📥 Received chunk #%d [%s] - Size: %d bytes\n", 
		chunk.ChunkNumber, chunk.Type, len(chunk.Data))
	
	// Verify checksum immediately
	if !fr.verifyChecksum(chunk) {
		fmt.Printf("❌ Checksum verification failed for chunk #%d - requesting retransmission\n", chunk.ChunkNumber)
		return fmt.Errorf("checksum verification failed for chunk #%d", chunk.ChunkNumber)
	}
	
	// Handle different chunk types
	switch chunk.Type {
	case TypeMetadata:
		return fr.handleMetadataChunk(chunk)
	case TypeData:
		return fr.handleDataChunk(chunk)
	case TypeEOF:
		return fr.handleEOFChunk(chunk)
	default:
		return fmt.Errorf("unknown chunk type: %s", chunk.Type)
	}
}

// verifyChecksum verifies the checksum of a received chunk
func (fr *FileReceiver) verifyChecksum(chunk *Chunk) bool {
	expectedChecksum := calculateChecksum(chunk.Data)
	return expectedChecksum == chunk.Checksum
}

// handleMetadataChunk processes metadata chunk and initializes file structure
func (fr *FileReceiver) handleMetadataChunk(chunk *Chunk) error {
	var metadata FileMetadata
	if err := json.Unmarshal(chunk.Data, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	
	fr.metadata = &metadata
	fr.expectedChunks = metadata.TotalChunks
	
	// Mark metadata chunk as received
	fr.chunks[chunk.ChunkNumber] = &ChunkState{
		Received:  true,
		Chunk:     chunk,
		Timestamp: time.Now().Unix(),
	}
	fr.receivedChunks++
	
	fmt.Printf("📋 Metadata received - File: %s, Size: %d bytes, Expected chunks: %d\n", 
		metadata.Filename, metadata.Size, metadata.TotalChunks)
	
	return nil
}

// handleDataChunk processes data chunks and buffers them
func (fr *FileReceiver) handleDataChunk(chunk *Chunk) error {
	// Check for duplicate chunks
	if state, exists := fr.chunks[chunk.ChunkNumber]; exists && state.Received {
		fmt.Printf("⚠️  Duplicate chunk #%d received - ignoring\n", chunk.ChunkNumber)
		return nil
	}
	
	// Store chunk
	fr.chunks[chunk.ChunkNumber] = &ChunkState{
		Received:  true,
		Chunk:     chunk,
		Timestamp: time.Now().Unix(),
	}
	fr.receivedChunks++
	
	fmt.Printf("✅ Data chunk #%d received and verified (%d/%d chunks)\n", 
		chunk.ChunkNumber, fr.receivedChunks, fr.expectedChunks)
	
	return nil
}

// handleEOFChunk processes EOF signal and triggers file assembly
func (fr *FileReceiver) handleEOFChunk(chunk *Chunk) error {
	// Mark EOF chunk as received
	fr.chunks[chunk.ChunkNumber] = &ChunkState{
		Received:  true,
		Chunk:     chunk,
		Timestamp: time.Now().Unix(),
	}
	fr.receivedChunks++
	
	fmt.Printf("🏁 EOF chunk received - initiating file assembly\n")
	
	// Check if we have all chunks
	if fr.receivedChunks == fr.expectedChunks {
		return fr.assembleFile()
	} else {
		fmt.Printf("⏳ Waiting for remaining chunks (%d/%d received)\n", 
			fr.receivedChunks, fr.expectedChunks)
		return nil
	}
}

// getMissingChunks returns a list of missing chunk numbers
func (fr *FileReceiver) getMissingChunks() []int {
	var missing []int
	
	for i := 1; i <= fr.expectedChunks; i++ {
		if state, exists := fr.chunks[i]; !exists || !state.Received {
			missing = append(missing, i)
		}
	}
	
	return missing
}

// assembleFile reassembles all chunks into the final file
func (fr *FileReceiver) assembleFile() error {
	if fr.metadata == nil {
		return fmt.Errorf("metadata not received")
	}
	
	fmt.Printf("🔧 Starting file assembly for: %s\n", fr.metadata.Filename)
	
	// Check for missing chunks
	missing := fr.getMissingChunks()
	if len(missing) > 0 {
		return fmt.Errorf("missing chunks: %v", missing)
	}
	
	// Create temporary file
	tempFile := filepath.Join(fr.tempDir, fr.metadata.Filename+".tmp")
	file, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()
	
	// Sort chunks by chunk number for sequential writing
	var chunkNumbers []int
	for chunkNum, state := range fr.chunks {
		if state.Received && state.Chunk.Type == TypeData {
			chunkNumbers = append(chunkNumbers, chunkNum)
		}
	}
	sort.Ints(chunkNumbers)
	
	// Write chunks sequentially
	for _, chunkNum := range chunkNumbers {
		chunk := fr.chunks[chunkNum].Chunk
		if _, err := file.Write(chunk.Data); err != nil {
			return fmt.Errorf("failed to write chunk #%d: %w", chunkNum, err)
		}
	}
	
	// Close file before hash verification
	file.Close()
	
	// Verify final file hash
	fileHash, err := calculateFileHash(tempFile)
	if err != nil {
		return fmt.Errorf("failed to calculate assembled file hash: %w", err)
	}
	
	if fileHash != fr.metadata.FileHash {
		os.Remove(tempFile) // Clean up corrupted file
		return fmt.Errorf("file hash mismatch - expected: %s, got: %s", fr.metadata.FileHash, fileHash)
	}
	
	// Atomic file completion - rename temp file to final name
	finalPath := filepath.Join(fr.outputPath, fr.metadata.Filename)
	if err := os.Rename(tempFile, finalPath); err != nil {
		return fmt.Errorf("failed to move temp file to final location: %w", err)
	}
	
	fr.isComplete = true
	
	fmt.Printf("🎉 File assembly completed successfully!\n")
	fmt.Printf("📁 File saved to: %s\n", finalPath)
	fmt.Printf("🔐 Hash verified: %s\n", fileHash)
	
	return nil
}

// GetProgress returns the current download progress
func (fr *FileReceiver) GetProgress() (int, int, float64) {
	fr.mutex.RLock()
	defer fr.mutex.RUnlock()
	
	if fr.expectedChunks == 0 {
		return 0, 0, 0.0
	}
	
	progress := float64(fr.receivedChunks) / float64(fr.expectedChunks) * 100
	return fr.receivedChunks, fr.expectedChunks, progress
}

// IsComplete returns whether the file assembly is complete
func (fr *FileReceiver) IsComplete() bool {
	fr.mutex.RLock()
	defer fr.mutex.RUnlock()
	return fr.isComplete
}

// createSampleFile creates a sample file for testing
func createSampleFile(filename string, size int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Write sample data
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(65 + (i % 26)) // A-Z pattern
	}
	
	_, err = file.Write(data)
	return err
}

func main() {
	// Create necessary directories
	tempDir := "temp"
	outputDir := "output"
	
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(outputDir, 0755)
	defer os.RemoveAll(tempDir)   // Clean up temp directory
	defer os.RemoveAll(outputDir) // Clean up output directory
	
	// Create a sample file for demonstration
	sampleFile := "sample_data.txt"
	sampleSize := 1024 // 1KB sample file
	
	fmt.Println("Creating sample file...")
	if err := createSampleFile(sampleFile, sampleSize); err != nil {
		fmt.Printf("Error creating sample file: %v\n", err)
		return
	}
	defer os.Remove(sampleFile) // Clean up
	
	// Create data processor with 256-byte buffer
	processor := NewDataProcessor(256)
	
	// Create file receiver
	receiver := NewFileReceiver(tempDir, outputDir)
	
	// Process the file (sender -> receiver simulation)
	fmt.Println("="*60)
	fmt.Println("STARTING FILE TRANSFER SIMULATION")
	fmt.Println("="*60)
	
	if err := processor.ProcessFile(sampleFile, receiver); err != nil {
		fmt.Printf("Error processing file: %v\n", err)
		return
	}
	
	// Check final status
	received, expected, progress := receiver.GetProgress()
	fmt.Printf("\n📊 Final Status: %d/%d chunks (%.1f%%) - Complete: %v\n", 
		received, expected, progress, receiver.IsComplete())
	
	fmt.Println("\n" + "="*60)
	fmt.Println("FILE TRANSFER COMPLETED SUCCESSFULLY")
	fmt.Println("="*60)
	
	// Demonstrate missing chunk scenario
	fmt.Println("\n🔄 Testing missing chunk scenario...")
	
	// Create another receiver for demonstration
	receiver2 := NewFileReceiver(tempDir, outputDir)
	processor2 := NewDataProcessor(128)
	
	// We'll simulate by not sending one of the chunks
	fmt.Println("(This would show missing chunk handling in a real P2P scenario)")
}