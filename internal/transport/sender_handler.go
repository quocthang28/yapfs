package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/processor"
	"yapfs/pkg/types"
	"yapfs/pkg/utils"
)

// SenderHandler implements MessageHandler for file sending operations
type SenderHandler struct {
	*BaseHandler

	// Configuration and dependencies
	config        *config.Config
	dataProcessor *processor.DataProcessor

	// File transfer state
	filePath      string
	fileMetadata  *types.FileMetadata
	transferState SenderState
	stateMutex    sync.RWMutex

	// Channel communication
	channel     *Channel
	sendDataCh  <-chan processor.DataChunk
	sendErrorCh <-chan error

	// Acknowledgement handling
	ackTimeouts map[MessageType]time.Duration
	ackReceived chan MessageType

	// Transfer control
	transferActive bool
	transferMutex  sync.RWMutex

	// Completion callback
	onCompleted func(error)
}

// NewSenderHandler creates a new sender handler
func NewSenderHandler(ctx context.Context, cfg *config.Config, filePath string, onCompleted func(error)) (*SenderHandler, error) {
	baseHandler := NewBaseHandler(ctx)

	handler := &SenderHandler{
		BaseHandler:   baseHandler,
		config:        cfg,
		dataProcessor: processor.NewDataProcessor(),
		filePath:      filePath,
		transferState: SenderInitializing,
		ackTimeouts: map[MessageType]time.Duration{
			MSG_METADATA_ACK:      30 * time.Second,
			MSG_TRANSFER_COMPLETE: 60 * time.Second,
		},
		ackReceived:    make(chan MessageType, 10),
		transferActive: false,
		onCompleted:    onCompleted,
	}

	// Prepare file for sending
	metadata, err := handler.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare file for sending: %w", err)
	}
	handler.fileMetadata = metadata

	// Start context monitoring for cancellation
	go handler.monitorContext()

	return handler, nil
}

// SetChannel sets the channel reference (called by Channel after creation)
func (s *SenderHandler) SetChannel(channel *Channel) {
	s.channel = channel
}

// HandleMessage processes incoming messages
func (s *SenderHandler) HandleMessage(msg Message, progressCh chan<- types.ProgressUpdate) error {
	log.Printf("Sender received message: %s", msg.Type)

	switch msg.Type {
	case MSG_READY:
		return s.handleReadyMessage(progressCh)
	case MSG_METADATA_ACK:
		return s.handleMetadataAck(msg)
	case MSG_TRANSFER_COMPLETE:
		return s.handleTransferComplete(msg)
	case MSG_ERROR:
		return s.handleErrorMessage(msg)
	default:
		log.Printf("Sender received unexpected message type: %s", msg.Type)
		return nil
	}
}

// OnChannelReady is called when the data channel is ready
func (s *SenderHandler) OnChannelReady() error {
	log.Printf("Sender channel ready, current state: %s", s.getState())
	s.setState(SenderWaitingForReady)

	// For sender channels, we don't send READY - we wait for receiver to send it
	return nil
}

// OnChannelClosed handles channel close events
func (s *SenderHandler) OnChannelClosed() {
	log.Printf("Sender channel closed")

	// Only transition to completed if we're not already in a terminal state
	currentState := s.getState()
	if currentState != SenderCompleted && currentState != SenderError {
		s.setState(SenderCompleted)
	}

	// Notify app layer of completion
	if s.onCompleted != nil {
		var err error
		if currentState == SenderError {
			err = fmt.Errorf("sender transfer failed")
		}
		s.onCompleted(err)
	}

	s.BaseHandler.OnChannelClosed()
}

// OnChannelError handles channel error events
func (s *SenderHandler) OnChannelError(err error) {
	log.Printf("Sender channel error: %v", err)

	// Only transition to error if we're not already in a terminal state
	currentState := s.getState()
	if currentState != SenderCompleted && currentState != SenderError {
		s.setState(SenderError)
	}

	// Clean up resources on error
	s.cleanup()

	// Notify app layer of error
	if s.onCompleted != nil {
		s.onCompleted(err)
	}

	s.BaseHandler.OnChannelError(err)
}

// handleReadyMessage processes READY message from receiver
func (s *SenderHandler) handleReadyMessage(progressCh chan<- types.ProgressUpdate) error {
	if s.getState() != SenderWaitingForReady {
		return fmt.Errorf("received READY message in invalid state: %s", s.getState())
	}

	log.Printf("Received READY from receiver, starting metadata transfer")
	s.setState(SenderSendingMetadata)

	// Send initial progress with metadata
	select {
	case progressCh <- types.ProgressUpdate{
		NewBytes: 0,
		MetaData: s.fileMetadata,
	}:
	default:
		log.Printf("Progress channel full, skipping metadata progress update")
	}

	// Send metadata
	return s.sendMetadata()
}

// handleMetadataAck processes METADATA_ACK message
func (s *SenderHandler) handleMetadataAck(msg Message) error {
	if s.getState() != SenderWaitingForMetadataAck {
		return fmt.Errorf("received METADATA_ACK in invalid state: %s", s.getState())
	}

	if msg.Error != "" {
		return fmt.Errorf("receiver rejected metadata: %s", msg.Error)
	}

	log.Printf("Metadata acknowledged by receiver, starting file transfer")
	s.setState(SenderTransferringData)

	// Start file data transfer
	return s.startFileTransfer()
}

// handleTransferComplete processes TRANSFER_COMPLETE message
func (s *SenderHandler) handleTransferComplete(msg Message) error {
	if s.getState() != SenderWaitingForCompletion {
		return fmt.Errorf("received TRANSFER_COMPLETE in invalid state: %s", s.getState())
	}

	if msg.Error != "" {
		return fmt.Errorf("receiver reported transfer error: %s", msg.Error)
	}

	log.Printf("Transfer completed successfully")
	s.setState(SenderCompleted)

	// Stop any ongoing transfer
	s.stopFileTransfer()

	// Signal completion by closing the channel gracefully
	if s.channel != nil {
		go func() {
			// Small delay to ensure any final messages are processed
			time.Sleep(100 * time.Millisecond)
			s.channel.Close()
		}()
	}

	return nil
}

// handleErrorMessage processes ERROR messages
func (s *SenderHandler) handleErrorMessage(msg Message) error {
	log.Printf("Received error from receiver: %s", msg.Error)
	s.setState(SenderError)
	s.stopFileTransfer()
	return fmt.Errorf("receiver error: %s", msg.Error)
}

// sendMetadata sends file metadata to receiver
func (s *SenderHandler) sendMetadata() error {
	metadataBytes, err := utils.EncodeJSON(s.fileMetadata)
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}

	err = s.channel.SendMessage(MSG_METADATA, metadataBytes)
	if err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	s.setState(SenderWaitingForMetadataAck)
	return nil
}

// startFileTransfer begins the file data transfer process
func (s *SenderHandler) startFileTransfer() error {
	s.transferMutex.Lock()
	if s.transferActive {
		s.transferMutex.Unlock()
		return fmt.Errorf("transfer already active")
	}
	s.transferActive = true
	s.transferMutex.Unlock()

	// Start reading file data
	dataCh, errCh := s.dataProcessor.StartReadingFile(s.config.WebRTC.ChunkSize)
	if dataCh == nil || errCh == nil {
		return fmt.Errorf("failed to start reading file")
	}

	s.sendDataCh = dataCh
	s.sendErrorCh = errCh

	// Start transfer goroutine
	go s.fileTransferLoop()

	return nil
}

// fileTransferLoop handles the main file transfer process
func (s *SenderHandler) fileTransferLoop() {
	defer func() {
		s.transferMutex.Lock()
		s.transferActive = false
		s.transferMutex.Unlock()
	}()

	for {
		select {
		case chunk, ok := <-s.sendDataCh:
			if !ok {
				log.Printf("Data channel closed unexpectedly")
				s.setState(SenderError)
				return
			}

			if chunk.EOF {
				if err := s.sendEOF(); err != nil {
					log.Printf("Error sending EOF: %v", err)
					s.setState(SenderError)
					return
				}
				s.setState(SenderWaitingForCompletion)
				return
			}

			if err := s.sendDataChunk(chunk); err != nil {
				log.Printf("Error sending data chunk: %v", err)
				s.setState(SenderError)
				return
			}

		case err, ok := <-s.sendErrorCh:
			if !ok {
				return
			}
			if err != nil {
				log.Printf("File reading error: %v", err)
				s.setState(SenderError)
				s.sendErrorToReceiver(fmt.Sprintf("file reading error: %v", err))
				return
			}

		case <-s.Context().Done():
			log.Printf("Transfer cancelled")
			return
		}
	}
}

// sendDataChunk sends a single data chunk
func (s *SenderHandler) sendDataChunk(chunk processor.DataChunk) error {
	err := s.channel.SendMessage(MSG_FILE_DATA, chunk.Data)
	if err != nil {
		return fmt.Errorf("failed to send data chunk: %w", err)
	}
	return nil
}

// sendEOF sends the end-of-file marker
func (s *SenderHandler) sendEOF() error {
	log.Printf("Sending EOF marker")
	err := s.channel.SendMessage(MSG_EOF, nil)
	if err != nil {
		return fmt.Errorf("failed to send EOF: %w", err)
	}
	return nil
}

// sendErrorToReceiver sends an error message to the receiver
func (s *SenderHandler) sendErrorToReceiver(errorMsg string) {
	msg := CreateControlMessage(MSG_ERROR, errorMsg)
	data, err := SerializeMessage(msg)
	if err != nil {
		log.Printf("Failed to serialize error message: %v", err)
		return
	}

	// Try to send error message (don't block if channel is full)
	select {
	case s.channel.outgoingMsgCh <- data:
		log.Printf("Sent error to receiver: %s", errorMsg)
	default:
		log.Printf("Could not send error to receiver (channel full): %s", errorMsg)
	}
}

// stopFileTransfer stops any ongoing file transfer
func (s *SenderHandler) stopFileTransfer() {
	s.transferMutex.Lock()
	defer s.transferMutex.Unlock()

	if s.transferActive {
		s.transferActive = false
		// Channels will be closed by dataProcessor cleanup
	}
}

// getState returns the current transfer state (thread-safe)
func (s *SenderHandler) getState() SenderState {
	s.stateMutex.RLock()
	defer s.stateMutex.RUnlock()
	return s.transferState
}

// setState updates the transfer state (thread-safe)
func (s *SenderHandler) setState(state SenderState) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if s.transferState != state {
		log.Printf("Sender state: %s -> %s", s.transferState, state)
		s.transferState = state
	}
}

// cleanup handles resource cleanup for sender
func (s *SenderHandler) cleanup() {
	// Stop any ongoing file transfer
	s.stopFileTransfer()
	
	// Close the data processor (closes file reader)
	if s.dataProcessor != nil {
		if err := s.dataProcessor.Close(); err != nil {
			log.Printf("Warning: failed to close data processor: %v", err)
		}
	}
}

// monitorContext monitors the context for cancellation and handles cleanup
func (s *SenderHandler) monitorContext() {
	<-s.Context().Done()
	
	// Context was cancelled (e.g., Ctrl+C)
	log.Printf("Sender context cancelled, cleaning up...")
	
	// Close the channel if it exists
	if s.channel != nil {
		s.channel.Close()
	}
}
