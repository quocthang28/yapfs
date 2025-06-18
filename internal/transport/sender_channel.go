package transport

import (
	"context"
	"fmt"
	"log"
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

	// Transfer state
	ctx        context.Context
	filePath   string
	totalBytes uint64
	metadata   *processor.FileMetadata
	bytesSent  uint64

	// Channels
	bufferControlCh chan struct{}
	progressCh      chan ProgressUpdate
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
func (s *SenderChannel) SetupFileSender(ctx context.Context, filePath string) (<-chan ProgressUpdate, error) {
	if s.dataChannel == nil {
		return nil, fmt.Errorf("data channel not created, call CreateFileSenderDataChannel first")
	}

	// Initialize transfer state
	s.ctx = ctx
	s.filePath = filePath
	s.bytesSent = 0
	s.bufferControlCh = make(chan struct{}, 3)
	s.progressCh = make(chan ProgressUpdate, 50)

	// Prepare file for sending
	err := s.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare file for sending: %w", err)
	}

	// Get file size and create metadata once
	s.totalBytes = uint64(s.dataProcessor.GetCurrentFileSize())

	metadataBytes, err := s.dataProcessor.CreateFileMetadata(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file metadata: %w", err)
	}

	s.metadata, err = s.dataProcessor.DecodeMetadata(metadataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode file metadata: %w", err)
	}

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	s.dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d",
			s.dataChannel.Label(), s.dataChannel.ID())
	})

	// Set up flow control
	s.dataChannel.SetBufferedAmountLowThreshold(s.config.WebRTC.BufferedAmountLowThreshold)
	s.dataChannel.OnBufferedAmountLow(func() {
		select {
		case s.bufferControlCh <- struct{}{}:
		default:
			log.Printf("Flow control: signal already pending, skipping")
		}
	})

	s.dataChannel.OnClose(func() {
		log.Printf("File transfer data channel closed")
		s.dataProcessor.Close()
	})

	s.dataChannel.OnError(func(err error) {
		log.Printf("File transfer data channel error: %v", err)
		s.dataProcessor.Close()
	})

	return s.progressCh, nil
}

// SendFile performs a blocking file transfer (call this after connection is established)
func (s *SenderChannel) SendFile() error {
	defer close(s.progressCh)

	// Wait for data channel to be ready
	for s.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
		select {
		case <-s.ctx.Done():
			return fmt.Errorf("cancelled while waiting for data channel: %v", s.ctx.Err())
		case <-time.After(100 * time.Millisecond):
			// Continue waiting
		}
	}

	log.Printf("Data channel ready, starting file transfer")

	// Send file metadata
	if err := s.sendMetadataPhase(); err != nil {
		return fmt.Errorf("error sending metadata: %v", err)
	}

	// Start file data transfer
	return s.sendFileDataPhase()
}

// sendMetadataPhase handles sending file metadata
func (s *SenderChannel) sendMetadataPhase() error {
	// Send initial progress with metadata (non-blocking)
	s.progressCh <- ProgressUpdate{
		BytesSent: 0,
		MetaData:  *s.metadata,
	}

	return s.sendFileMetaData()
}

// sendFileDataPhase handles the main file data transfer loop
func (s *SenderChannel) sendFileDataPhase() error {
	// Start file transfer
	dataCh, errCh := s.dataProcessor.StartReadingFile(s.config.WebRTC.ChunkSize)
	if dataCh == nil || errCh == nil {
		return fmt.Errorf("no file prepared for transfer")
	}

	// Process data chunks
	for {
		select {
		case chunk, ok := <-dataCh:
			if !ok {
				// Channel closed unexpectedly
				return fmt.Errorf("data channel closed unexpectedly")
			}

			if chunk.EOF {
				return s.sendEOFPhase()
			}

			if err := s.sendDataChunk(chunk); err != nil {
				return err
			}

			if err := s.handleFlowControl(); err != nil {
				return err
			}

		case err, ok := <-errCh:
			if !ok {
				// Error channel closed, no more errors expected
				return nil
			}
			if err != nil {
				return fmt.Errorf("error during file transfer: %v", err)
			}

		case <-s.ctx.Done():
			return fmt.Errorf("file transfer cancelled: %v", s.ctx.Err())
		}
	}
}

// sendDataChunk sends a single data chunk and updates progress
func (s *SenderChannel) sendDataChunk(chunk processor.DataChunk) error {
	// Send data chunk
	err := s.dataChannel.Send(chunk.Data)
	if err != nil {
		return fmt.Errorf("error sending data: %v", err)
	}

	// Update progress tracking
	s.bytesSent += uint64(len(chunk.Data))
	s.updateProgress()
	return nil
}

// sendEOFPhase handles EOF signaling and cleanup
func (s *SenderChannel) sendEOFPhase() error {
	// Send EOF marker
	err := s.dataChannel.Send([]byte("EOF"))
	if err != nil {
		return fmt.Errorf("error sending EOF: %v", err)
	}

	log.Printf("File transfer complete")

	// Send final progress (non-blocking)
	update := ProgressUpdate{
		BytesSent: s.bytesSent,
		MetaData:  *s.metadata,
	}

	select {
	case s.progressCh <- update:
		// Progress sent successfully
	default:
		// Progress channel full, skip this update
	}

	// Close the channel after sending EOF
	err = s.dataChannel.GracefulClose()
	if err != nil {
		return fmt.Errorf("error closing channel: %v", err)
	}

	return nil
}

// updateProgress sends raw progress data (UI layer handles calculations)
func (s *SenderChannel) updateProgress() {
	// Send raw progress data - let UI decide when/how to display
	update := ProgressUpdate{
		BytesSent: s.bytesSent,
		MetaData:  *s.metadata,
	}
	// Non-blocking progress send to prevent data channel blocking
	select {
	case s.progressCh <- update:
		// Progress sent successfully
	default:
		// Progress channel full, skip this update to avoid blocking data transfer
	}
}

// handleFlowControl manages flow control and backpressure
func (s *SenderChannel) handleFlowControl() error {
	// Flow control: wait if buffer is too full
	if s.dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
		select {
		case <-s.bufferControlCh:
			return nil
		case <-s.ctx.Done():
			return fmt.Errorf("file transfer cancelled: %v", s.ctx.Err())
		case <-time.After(30 * time.Second):
			return fmt.Errorf("flow control timeout - WebRTC channel may be dead")
		}
	}
	return nil
}

func (s *SenderChannel) sendFileMetaData() error {
	metadataBytes, err := s.dataProcessor.CreateFileMetadata(s.filePath)
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
