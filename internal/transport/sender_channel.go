package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// SenderChannel manages data channel operations for sending files
type SenderChannel struct {
	config        *config.Config
	dataChannel   *webrtc.DataChannel
	dataProcessor *processor.DataProcessor
}

// NewSenderChannel creates a new data channel sender
func NewSenderChannel(cfg *config.Config) *SenderChannel {
	return &SenderChannel{
		config:        cfg,
		dataProcessor: processor.NewDataProcessor(),
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files and stores it internally
func (s *SenderChannel) CreateFileSenderDataChannel(peerConn *webrtc.PeerConnection, label string) error {
	ordered := true //TODO: once data processor handle chunking this should be false

	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := peerConn.CreateDataChannel(label, options)
	if err != nil {
		return fmt.Errorf("failed to create file data channel: %w", err)
	}

	s.dataChannel = dataChannel
	return nil
}

// SetupFileSender configures file sending with progress reporting
func (s *SenderChannel) SetupFileSender(ctx context.Context, filePath string) (<-chan struct{}, <-chan ProgressUpdate, error) {
	if s.dataChannel == nil {
		return nil, nil, fmt.Errorf("data channel not created, call CreateFileSenderDataChannel first")
	}

	// Flow control channel: signals when WebRTC buffer has space for more data
	// Buffered (size 3) to handle multiple OnBufferedAmountLow events without blocking
	sendMoreCh := make(chan struct{}, 3)
	
	// Completion signal channel: closed when file transfer finishes (success or error)
	doneCh := make(chan struct{})
	
	// Progress updates channel: sends transfer statistics to UI layer
	// Buffered to prevent blocking on progress reporting
	progressCh := make(chan ProgressUpdate, 50)
	
	// Ensures doneCh is closed exactly once, preventing panic from multiple close attempts
	var doneOnce sync.Once

	// Prepare file for sending
	err := s.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare file for sending: %w", err)
	}

	// Get file size for progress calculation
	totalBytes := uint64(s.dataProcessor.GetCurrentFileSize())

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	s.dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d. Sending metadata first...",
			s.dataChannel.Label(), s.dataChannel.ID())

		go s.startFileTransfer(ctx, filePath, totalBytes, sendMoreCh, doneCh, progressCh, &doneOnce)
	})

	// Set up flow control
	s.dataChannel.SetBufferedAmountLowThreshold(s.config.WebRTC.BufferedAmountLowThreshold)
	s.dataChannel.OnBufferedAmountLow(func() {
		// Use non-blocking send with increased buffer to reduce dropped signals
		// If buffer is full, it means we already have pending signals
		select {
		case sendMoreCh <- struct{}{}:
			// Signal sent successfully
		default:
			// Buffer full - signal already pending, no need to add more
			log.Printf("Flow control: signal already pending, skipping")
		}
	})

	return doneCh, progressCh, nil
}

// FileTransferContext contains context for file transfer operations
type FileTransferContext struct {
	ctx        context.Context
	filePath   string
	totalBytes uint64
	sendMoreCh chan struct{}
	doneCh     chan struct{}
	progressCh chan ProgressUpdate
	doneOnce   *sync.Once

	// Progress tracking
	bytesSent uint64
	metadata  *processor.FileMetadata
}

// startFileTransfer manages the complete file transfer process
func (s *SenderChannel) startFileTransfer(ctx context.Context, filePath string, totalBytes uint64, sendMoreCh chan struct{}, doneCh chan struct{}, progressCh chan ProgressUpdate, doneOnce *sync.Once) {
	defer close(progressCh) // Close progress channel when done

	transferCtx := &FileTransferContext{
		ctx:              ctx,
		filePath:         filePath,
		totalBytes:       totalBytes,
		sendMoreCh:       sendMoreCh,
		doneCh:           doneCh,
		progressCh:       progressCh,
		doneOnce:         doneOnce,
		bytesSent: 0,
	}

	// Send file metadata
	if err := s.sendMetadataPhase(transferCtx); err != nil {
		log.Printf("Error sending metadata: %v", err)
		transferCtx.doneOnce.Do(func() { close(transferCtx.doneCh) })
		return
	}

	// Start file data transfer
	s.sendFileDataPhase(transferCtx)
}

// sendMetadataPhase handles sending file metadata
func (s *SenderChannel) sendMetadataPhase(ctx *FileTransferContext) error {
	// Create metadata for progress update
	metadataBytes, err := s.dataProcessor.CreateFileMetadata(ctx.filePath)
	if err != nil {
		return fmt.Errorf("error creating file metadata: %w", err)
	}
	
	metadata, err := s.dataProcessor.DecodeMetadata(metadataBytes)
	if err != nil {
		return fmt.Errorf("error decoding file metadata: %w", err)
	}

	// Store metadata in context for subsequent progress updates
	ctx.metadata = metadata

	// Send initial progress with metadata
	ctx.progressCh <- ProgressUpdate{
		BytesSent: 0,
		MetaData:  *metadata,
	}

	return s.sendFileMetaData(ctx.filePath)
}

// sendFileDataPhase handles the main file data transfer loop
func (s *SenderChannel) sendFileDataPhase(ctx *FileTransferContext) {
	// Start file transfer
	dataCh, errCh := s.dataProcessor.StartReadingFile(s.config.WebRTC.ChunkSize)
	if dataCh == nil || errCh == nil {
		log.Printf("No file prepared for transfer")
		ctx.doneOnce.Do(func() { close(ctx.doneCh) })
		return
	}

	// Process data chunks
	for {
		select {
		case chunk, ok := <-dataCh:
			if !ok {
				// Channel closed unexpectedly
				log.Printf("Data channel closed unexpectedly")
				ctx.doneOnce.Do(func() { close(ctx.doneCh) })
				return
			}

			if chunk.EOF {
				s.sendEOFPhase(ctx)
				return
			}

			if !s.sendDataChunk(ctx, chunk) {
				return // Error occurred, transfer terminated
			}

			if !s.handleFlowControl(ctx) {
				return // Flow control timeout or cancellation
			}

		case err, ok := <-errCh:
			if !ok {
				// Error channel closed, no more errors expected
				return
			}
			if err != nil {
				log.Printf("Error during file transfer: %v", err)
				ctx.doneOnce.Do(func() { close(ctx.doneCh) })
				return
			}

		case <-ctx.ctx.Done():
			log.Printf("File transfer cancelled: %v", ctx.ctx.Err())
			ctx.doneOnce.Do(func() { close(ctx.doneCh) })
			return
		}
	}
}

// sendDataChunk sends a single data chunk and updates progress
func (s *SenderChannel) sendDataChunk(ctx *FileTransferContext, chunk processor.DataChunk) bool {
	// Send data chunk
	err := s.dataChannel.Send(chunk.Data)
	if err != nil {
		log.Printf("Error sending data: %v", err)
		ctx.doneOnce.Do(func() { close(ctx.doneCh) })
		return false
	}

	// Update progress tracking
	ctx.bytesSent += uint64(len(chunk.Data))
	s.updateProgress(ctx)
	return true
}

// sendEOFPhase handles EOF signaling and cleanup
func (s *SenderChannel) sendEOFPhase(ctx *FileTransferContext) {
	// Send EOF marker
	err := s.dataChannel.Send([]byte("EOF"))
	if err != nil {
		log.Printf("Error sending EOF: %v", err)
	} else {
		log.Printf("File transfer complete")
	}

	// Send final progress
	update := ProgressUpdate{
		BytesSent: ctx.bytesSent,
	}
	if ctx.metadata != nil {
		update.MetaData = *ctx.metadata
	}
	ctx.progressCh <- update

	// Close the channel after sending EOF
	err = s.dataChannel.GracefulClose()
	if err != nil {
		log.Printf("Error closing channel: %v", err)
	}

	ctx.doneOnce.Do(func() { close(ctx.doneCh) })
}

// updateProgress sends raw progress data (UI layer handles calculations)
func (s *SenderChannel) updateProgress(ctx *FileTransferContext) {
	// Send raw progress data - let UI decide when/how to display
	update := ProgressUpdate{
		BytesSent: ctx.bytesSent,
	}
	if ctx.metadata != nil {
		update.MetaData = *ctx.metadata
	}
	ctx.progressCh <- update
}

// handleFlowControl manages flow control and backpressure
func (s *SenderChannel) handleFlowControl(ctx *FileTransferContext) bool {
	// Flow control: wait if buffer is too full
	if s.dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
		select {
		case <-ctx.sendMoreCh:
			return true
		case <-ctx.ctx.Done():
			log.Printf("File transfer cancelled: %v", ctx.ctx.Err())
			ctx.doneOnce.Do(func() { close(ctx.doneCh) })
			return false
		case <-time.After(30 * time.Second):
			log.Printf("Flow control timeout - WebRTC channel may be dead")
			ctx.doneOnce.Do(func() { close(ctx.doneCh) })
			return false
		}
	}
	return true
}

func (s *SenderChannel) sendFileMetaData(filePath string) error {
	metadataBytes, err := s.dataProcessor.CreateFileMetadata(filePath)
	if err != nil {
		return fmt.Errorf("error creating file metadata: %w", err)
	}

	// Send metadata with "METADATA:" prefix
	metadataMsg := append([]byte("METADATA:"), metadataBytes...)
	err = s.dataChannel.Send(metadataMsg)
	if err != nil {
		return fmt.Errorf("error sending metadata: %w", err)
	}

	return nil
}

// Close cleans up the SenderChannel resources
func (s *SenderChannel) Close() error {
	if s.dataProcessor != nil {
		return s.dataProcessor.Close()
	}
	return nil
}
