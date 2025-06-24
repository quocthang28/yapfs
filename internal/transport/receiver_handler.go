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

// ReceiverHandler implements MessageHandler for file receiving operations
type ReceiverHandler struct {
	*BaseHandler

	// Configuration and dependencies
	config        *config.Config
	dataProcessor *processor.DataProcessor

	// File transfer state
	DestPath         string // Public field for destination path configuration
	fileMetadata     *types.FileMetadata
	transferState    ReceiverState
	stateMutex       sync.RWMutex
	metadataReceived bool

	// Channel communication
	channel *Channel

	// Transfer tracking
	bytesReceived uint64
	transferMutex sync.RWMutex
	
	// Completion callback
	onCompleted func(error)
}

// NewReceiverHandler creates a new receiver handler
func NewReceiverHandler(ctx context.Context, cfg *config.Config, destPath string, onCompleted func(error)) *ReceiverHandler {
	baseHandler := NewBaseHandler(ctx)

	return &ReceiverHandler{
		BaseHandler:      baseHandler,
		config:           cfg,
		dataProcessor:    processor.NewDataProcessor(),
		DestPath:         destPath,
		transferState:    ReceiverInitializing,
		metadataReceived: false,
		bytesReceived:    0,
		onCompleted:      onCompleted,
	}
}

// SetChannel sets the channel reference (called by Channel after creation)
func (r *ReceiverHandler) SetChannel(channel *Channel) {
	r.channel = channel
}

// HandleMessage processes incoming messages
func (r *ReceiverHandler) HandleMessage(msg Message, progressCh chan<- types.ProgressUpdate) error {
	switch msg.Type {
	case MSG_METADATA:
		return r.handleMetadataMessage(msg, progressCh)
	case MSG_FILE_DATA:
		return r.handleFileDataMessage(msg, progressCh)
	case MSG_EOF:
		return r.handleEOFMessage(progressCh)
	case MSG_ERROR:
		return r.handleErrorMessage(msg)
	default:
		log.Printf("Receiver received unexpected message type: %s", msg.Type)
		return nil
	}
}

// OnChannelReady is called when the data channel is ready
func (r *ReceiverHandler) OnChannelReady() error {
	log.Printf("Receiver channel ready, sending READY signal")
	r.setState(ReceiverReady)

	// Send READY message to sender
	err := r.channel.SendMessage(MSG_READY, nil)
	if err != nil {
		log.Printf("Failed to send READY message: %v", err)
		return err
	}
	
	log.Printf("READY message sent successfully")
	return nil
}

// OnChannelClosed handles channel close events
func (r *ReceiverHandler) OnChannelClosed() {
	log.Printf("Receiver channel closed")
	
	// Only transition to completed if we're not already in a terminal state
	currentState := r.getState()
	if currentState != ReceiverCompleted && currentState != ReceiverError {
		r.setState(ReceiverCompleted)
	}
	
	// Notify app layer of completion
	if r.onCompleted != nil {
		var err error
		if currentState == ReceiverError {
			err = fmt.Errorf("receiver transfer failed")
		}
		r.onCompleted(err)
	}
	
	r.BaseHandler.OnChannelClosed()
}

// OnChannelError handles channel error events
func (r *ReceiverHandler) OnChannelError(err error) {
	log.Printf("Receiver channel error: %v", err)
	
	// Only transition to error if we're not already in a terminal state
	currentState := r.getState()
	if currentState != ReceiverCompleted && currentState != ReceiverError {
		r.setState(ReceiverError)
	}
	
	// Notify app layer of error
	if r.onCompleted != nil {
		r.onCompleted(err)
	}
	
	r.BaseHandler.OnChannelError(err)
}

// handleMetadataMessage processes METADATA messages
func (r *ReceiverHandler) handleMetadataMessage(msg Message, progressCh chan<- types.ProgressUpdate) error {
	if r.getState() != ReceiverReady {
		return r.sendErrorAndFail(fmt.Sprintf("received METADATA in invalid state: %s", r.getState()))
	}

	if r.metadataReceived {
		return r.sendErrorAndFail("metadata already received")
	}

	// Parse metadata
	metadata, err := utils.DecodeJSON[types.FileMetadata](msg.Payload)
	if err != nil {
		return r.sendErrorAndFail(fmt.Sprintf("failed to decode metadata: %v", err))
	}

	log.Printf("Received metadata: %s (size: %d bytes, type: %s)",
		metadata.Name, metadata.Size, metadata.MimeType)

	r.fileMetadata = &metadata
	r.metadataReceived = true
	r.setState(ReceiverPreparingFile)

	// Send initial progress with metadata
	select {
	case progressCh <- types.ProgressUpdate{
		NewBytes: 0,
		MetaData: &metadata,
	}:
	default:
		log.Printf("Progress channel full, skipping metadata progress update")
	}

	// Prepare file for receiving
	finalPath, err := r.dataProcessor.PrepareFileForReceiving(r.DestPath, &metadata)
	if err != nil {
		return r.sendErrorAndFail(fmt.Sprintf("failed to prepare file for receiving: %v", err))
	}

	log.Printf("Ready to receive file to: %s", finalPath)
	r.setState(ReceiverReceivingData)

	// Send metadata acknowledgment
	return r.channel.SendMessage(MSG_METADATA_ACK, nil)
}

// handleFileDataMessage processes FILE_DATA messages
func (r *ReceiverHandler) handleFileDataMessage(msg Message, progressCh chan<- types.ProgressUpdate) error {
	if r.getState() != ReceiverReceivingData {
		return r.sendErrorAndFail(fmt.Sprintf("received FILE_DATA in invalid state: %s", r.getState()))
	}

	if !r.metadataReceived {
		return r.sendErrorAndFail("received file data before metadata")
	}

	// Write data using DataProcessor
	err := r.dataProcessor.WriteData(msg.Payload)
	if err != nil {
		return r.sendErrorAndFail(fmt.Sprintf("failed to write data: %v", err))
	}

	// Update bytes received counter
	r.transferMutex.Lock()
	r.bytesReceived += uint64(len(msg.Payload))
	currentBytes := r.bytesReceived
	r.transferMutex.Unlock()

	// Send progress update
	update := types.ProgressUpdate{
		NewBytes: uint64(len(msg.Payload)),
	}

	select {
	case progressCh <- update:
	default:
		// Progress channel full, skip this update to avoid blocking
	}

	// Optional: Log progress for large files
	if r.fileMetadata != nil && r.fileMetadata.Size > 0 {
		progress := float64(currentBytes) / float64(r.fileMetadata.Size) * 100
		if currentBytes%1048576 == 0 { // Log every MB
			log.Printf("Received %.1f%% (%d/%d bytes)", progress, currentBytes, r.fileMetadata.Size)
		}
	}

	return nil
}

// handleEOFMessage processes EOF messages
func (r *ReceiverHandler) handleEOFMessage(_ chan<- types.ProgressUpdate) error {
	if r.getState() != ReceiverReceivingData {
		return r.sendErrorAndFail(fmt.Sprintf("received EOF in invalid state: %s", r.getState()))
	}

	log.Printf("Received EOF, finishing file transfer")

	// Finish receiving and get total bytes
	totalBytes, err := r.dataProcessor.FinishReceiving()
	if err != nil {
		return r.sendErrorAndFail(fmt.Sprintf("failed to finish receiving: %v", err))
	}

	log.Printf("File transfer complete: %d bytes received", totalBytes)
	r.setState(ReceiverCompleted)

	// Verify file size matches metadata if available
	if r.fileMetadata != nil && int64(totalBytes) != r.fileMetadata.Size {
		return r.sendErrorAndFail(fmt.Sprintf("file size mismatch: expected %d, got %d",
			r.fileMetadata.Size, totalBytes))
	}

	// Send transfer complete acknowledgment
	if err := r.channel.SendMessage(MSG_TRANSFER_COMPLETE, nil); err != nil {
		return err
	}

	// Signal completion by closing the channel gracefully after a short delay
	go func() {
		// Allow time for the TRANSFER_COMPLETE message to be sent and processed
		time.Sleep(200 * time.Millisecond)
		if r.channel != nil {
			r.channel.Close()
		}
	}()

	return nil
}

// handleErrorMessage processes ERROR messages
func (r *ReceiverHandler) handleErrorMessage(msg Message) error {
	log.Printf("Received error from sender: %s", msg.Error)
	r.setState(ReceiverError)
	return fmt.Errorf("sender error: %s", msg.Error)
}

// sendErrorAndFail sends an error message to sender and sets error state
func (r *ReceiverHandler) sendErrorAndFail(errorMsg string) error {
	log.Printf("Receiver error: %s", errorMsg)
	r.setState(ReceiverError)

	// Try to send error message to sender
	if r.channel != nil {
		err := r.channel.SendMessage(MSG_ERROR, []byte(errorMsg))
		if err != nil {
			log.Printf("Failed to send error message to sender: %v", err)
		}
	}

	return fmt.Errorf("%s", errorMsg)
}

// getState returns the current transfer state (thread-safe)
func (r *ReceiverHandler) getState() ReceiverState {
	r.stateMutex.RLock()
	defer r.stateMutex.RUnlock()
	return r.transferState
}

// setState updates the transfer state (thread-safe)
func (r *ReceiverHandler) setState(state ReceiverState) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()

	if r.transferState != state {
		log.Printf("Receiver state: %s -> %s", r.transferState, state)
		r.transferState = state
	}
}

// GetFileMetadata returns the received file metadata
func (r *ReceiverHandler) GetFileMetadata() *types.FileMetadata {
	return r.fileMetadata
}

// GetCurrentState returns the current transfer state
func (r *ReceiverHandler) GetCurrentState() ReceiverState {
	return r.getState()
}

// GetBytesReceived returns the number of bytes received so far
func (r *ReceiverHandler) GetBytesReceived() uint64 {
	r.transferMutex.RLock()
	defer r.transferMutex.RUnlock()
	return r.bytesReceived
}

// IsMetadataReceived returns whether metadata has been received
func (r *ReceiverHandler) IsMetadataReceived() bool {
	return r.metadataReceived
}
