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
	ctx      context.Context
	metadata *processor.FileMetadata

	// Channels
	bufferControlCh chan struct{}
}

// NewSenderChannel creates a new data channel sender
func NewSenderChannel(cfg *config.Config) *SenderChannel {
	return &SenderChannel{
		config:        cfg,
		dataProcessor: processor.NewDataProcessor(),
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files and initializes everything needed for transfer
func (s *SenderChannel) CreateFileSenderDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection, label string, filePath string) error {
	ordered := true //TODO: once data processor handle chunking this should be false

	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := peerConn.CreateDataChannel(label, options)
	if err != nil {
		return fmt.Errorf("failed to create file data channel: %w", err)
	}

	s.dataChannel = dataChannel

	// Initialize transfer state
	s.ctx = ctx
	s.bufferControlCh = make(chan struct{}, 3)

	// Prepare file for sending and get metadata
	s.metadata, err = s.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return fmt.Errorf("failed to prepare file for sending: %w", err)
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

	return nil
}

// SendFile performs a non-blocking file transfer, returns progress channel immediately
func (s *SenderChannel) SendFile() (<-chan ProgressUpdate, error) {
	progressCh := make(chan ProgressUpdate, 50)

	// Start file transfer in a goroutine
	go func() {
		defer close(progressCh)

		// Wait for data channel to be ready
		for s.dataChannel.ReadyState() != webrtc.DataChannelStateOpen {
			select {
			case <-s.ctx.Done():
				log.Printf("Cancelled while waiting for data channel: %v", s.ctx.Err())
				return
			case <-time.After(100 * time.Millisecond):
				// Continue waiting
			}
		}

		log.Printf("Data channel ready, starting file transfer")

		// Send file metadata
		if err := s.sendMetadataPhase(progressCh); err != nil {
			log.Printf("Error sending metadata: %v", err)
			return
		}

		// Start file data transfer
		if err := s.sendFileDataPhase(progressCh); err != nil {
			log.Printf("Error during file transfer: %v", err)
			return
		}
	}()

	return progressCh, nil
}

// sendMetadataPhase handles sending file metadata
func (s *SenderChannel) sendMetadataPhase(progressCh chan<- ProgressUpdate) error {
	// Send initial progress with metadata (non-blocking)
	progressCh <- ProgressUpdate{
		NewBytes: 0,
		MetaData: *s.metadata,
	}

	metadataBytes, err := s.dataProcessor.EncodeMetadata(s.metadata)
	if err != nil {
		return fmt.Errorf("error encoding file metadata: %w", err)
	}

	// Send metadata with "METADATA:" prefix
	metadataMsg := append([]byte("METADATA:"), metadataBytes...)
	err = s.dataChannel.Send(metadataMsg)
	if err != nil {
		return fmt.Errorf("error sending metadata: %w", err)
	}

	return nil
}

// sendFileDataPhase handles the main file data transfer loop
func (s *SenderChannel) sendFileDataPhase(progressCh chan<- ProgressUpdate) error {
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
				return s.sendEOFPhase(progressCh)
			}

			if err := s.sendDataChunk(chunk, progressCh); err != nil {
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
func (s *SenderChannel) sendDataChunk(chunk processor.DataChunk, progressCh chan<- ProgressUpdate) error {
	// Send data chunk
	err := s.dataChannel.Send(chunk.Data)
	if err != nil {
		return fmt.Errorf("error sending data: %v", err)
	}

	s.updateProgress(len(chunk.Data), progressCh)

	return nil
}

// sendEOFPhase handles EOF signaling and cleanup
func (s *SenderChannel) sendEOFPhase(progressCh chan<- ProgressUpdate) error {
	// Send EOF marker
	err := s.dataChannel.Send([]byte("EOF"))
	if err != nil {
		return fmt.Errorf("error sending EOF: %v", err)
	}

	log.Printf("File transfer complete")

	// Send final progress with 0 new bytes (non-blocking)
	update := ProgressUpdate{
		NewBytes: 0,
		MetaData: *s.metadata,
	}

	select {
	case progressCh <- update:
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
func (s *SenderChannel) updateProgress(newbytes int, progressCh chan<- ProgressUpdate) {
	// Send raw progress data - let UI decide when/how to display
	update := ProgressUpdate{
		NewBytes: uint64(newbytes),
		MetaData: *s.metadata,
	}

	// Non-blocking progress send to prevent data channel blocking
	select {
	case progressCh <- update:
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
