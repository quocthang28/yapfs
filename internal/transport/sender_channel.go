package transport

import (
	"context"
	"fmt"
	"log"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/processor"
	"yapfs/pkg/types"
	"yapfs/pkg/utils"

	"github.com/pion/webrtc/v4"
)

// SenderChannel manages data channel operations for sending files
type SenderChannel struct {
	ctx             context.Context
	config          *config.Config
	dataChannel     *webrtc.DataChannel
	dataProcessor   *processor.DataProcessor
	metadata        *types.FileMetadata // TODO: remove this
	bufferControlCh chan struct{}       // Signals when WebRTC buffer is ready for more data (flow control)
	readyCh         chan struct{}       // Signals when data channel is open and ready for file transfer
}

// NewSenderChannel creates a new data channel sender
func NewSenderChannel(cfg *config.Config) *SenderChannel {
	return &SenderChannel{
		config:          cfg,
		dataProcessor:   processor.NewDataProcessor(),
		bufferControlCh: make(chan struct{}),
		readyCh:         make(chan struct{}),
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files and initializes everything needed for transfer
func (s *SenderChannel) CreateFileSenderDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection, label string, filePath string) error {
	s.ctx = ctx

	ordered := true

	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := peerConn.CreateDataChannel(label, options)
	if err != nil {
		return fmt.Errorf("failed to create file data channel: %w", err)
	}

	s.dataChannel = dataChannel

	// Prepare file for sending and get metadata
	s.metadata, err = s.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return fmt.Errorf("failed to prepare file for sending: %w", err)
	}

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	s.dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d", s.dataChannel.Label(), s.dataChannel.ID())
		close(s.readyCh)
	})

	// Set up flow control
	s.dataChannel.SetBufferedAmountLowThreshold(s.config.WebRTC.BufferedAmountLowThreshold)
	s.dataChannel.OnBufferedAmountLow(func() {
		select {
		case s.bufferControlCh <- struct{}{}:
		default:
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
func (s *SenderChannel) SendFile() (<-chan types.ProgressUpdate, error) {
	progressCh := make(chan types.ProgressUpdate, 50)

	// Start file transfer in a goroutine
	go func() {
		defer close(progressCh)

		// Wait for data channel to be ready
		select {
		case <-s.readyCh:
			log.Printf("Data channel ready, starting file transfer")
		case <-s.ctx.Done():
			log.Printf("Cancelled while waiting for data channel: %v", s.ctx.Err())
			return
		}

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
func (s *SenderChannel) sendMetadataPhase(progressCh chan<- types.ProgressUpdate) error {
	// Send initial progress with metadata (non-blocking)
	progressCh <- types.ProgressUpdate{
		NewBytes: 0,
		MetaData: s.metadata,
	}

	metadataBytes, err := utils.EncodeJSON(s.metadata)
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
func (s *SenderChannel) sendFileDataPhase(progressCh chan<- types.ProgressUpdate) error {
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
				return s.sendEOF()
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
func (s *SenderChannel) sendDataChunk(chunk processor.DataChunk, progressCh chan<- types.ProgressUpdate) error {
	// Send data chunk
	err := s.dataChannel.Send(chunk.Data)
	if err != nil {
		return fmt.Errorf("error sending data: %v", err)
	}

	// Send progress update
	update := types.ProgressUpdate{
		NewBytes: uint64(len(chunk.Data)),
	}

	select {
	case progressCh <- update:
		// Progress sent successfully
	default:
		// Progress channel full, skip this update to avoid blocking data transfer
	}
	
	return nil
}

// sendEOF handles EOF signaling and cleanup
func (s *SenderChannel) sendEOF() error {
	// Send EOF marker
	err := s.dataChannel.Send([]byte("EOF"))
	if err != nil {
		return fmt.Errorf("error sending EOF: %v", err)
	}

	// Close the channel after sending EOF
	err = s.dataChannel.GracefulClose()
	if err != nil {
		return fmt.Errorf("error closing channel: %v", err)
	}

	return nil
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