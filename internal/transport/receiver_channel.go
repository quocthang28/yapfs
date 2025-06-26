package transport

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"

	"yapfs/internal/config"
	"yapfs/internal/processor"
	"yapfs/pkg/types"
	"yapfs/pkg/utils"

	"github.com/pion/webrtc/v4"
)

// ReceiverChannel manages data channel operations for receiving files
type ReceiverChannel struct {
	ctx              context.Context
	config           *config.Config
	dataChannel      *webrtc.DataChannel
	dataProcessor    *processor.DataProcessor
	destPath         string
	readyCh          chan struct{} // Signals when data channel is open and ready for file transfer
	doneCh           chan struct{} // Signals when file transfer is complete
	progressCh       chan types.ProgressUpdate
	metadataReceived bool // Track if metadata has been received

	// Progress tracking
	fileMetadata *types.FileMetadata

	// Synchronization
	doneOnce sync.Once
}

// NewReceiverChannel creates a new data channel receiver
func NewReceiverChannel(cfg *config.Config) *ReceiverChannel {
	return &ReceiverChannel{
		config:           cfg,
		dataProcessor:    processor.NewDataProcessor(),
		readyCh:          make(chan struct{}),
		doneCh:           make(chan struct{}),
		metadataReceived: false,
	}
}

// SetupFileReceiver sets up handlers for receiving files
func (r *ReceiverChannel) SetupFileReceiver(ctx context.Context, peerConn *webrtc.PeerConnection, destPath string) error {
	r.ctx = ctx
	r.destPath = destPath

	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		r.dataChannel = dataChannel
		log.Printf("Received data channel: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

		r.dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d. Waiting for metadata...", r.dataChannel.Label(), r.dataChannel.ID())
			close(r.readyCh)
		})

		r.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			r.handleMessage(msg)
		})

		r.dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
			r.dataProcessor.Close()
		})

		r.dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			r.dataProcessor.Close()
		})
	})

	return nil
}

// ReceiveFile performs a non-blocking file receive, returns progress channel immediately
func (r *ReceiverChannel) ReceiveFile() (<-chan types.ProgressUpdate, error) {
	r.progressCh = make(chan types.ProgressUpdate, 50)

	// Start file receive in a goroutine
	go func() {
		defer close(r.progressCh)

		// Wait for data channel to be ready
		select {
		case <-r.readyCh:
			log.Printf("Data channel ready, waiting for file transfer")
		case <-r.ctx.Done():
			log.Printf("Cancelled while waiting for data channel: %v", r.ctx.Err())
			return
		}

		// Wait for completion
		select {
		case <-r.doneCh:
			log.Printf("File transfer completed")
		case <-r.ctx.Done():
			log.Printf("File transfer cancelled: %v", r.ctx.Err())
		}
	}()

	return r.progressCh, nil
}

// ClearPartialFile removes any partially written file
func (r *ReceiverChannel) ClearPartialFile() error {
	if r.dataProcessor != nil {
		return r.dataProcessor.ClearPartialFile()
	}

	return nil
}

// handleMessage dispatches messages to appropriate handlers based on type
func (r *ReceiverChannel) handleMessage(msg webrtc.DataChannelMessage) {
	// Determine message type and dispatch to appropriate handler
	switch {
	case !r.metadataReceived && bytes.HasPrefix(msg.Data, []byte("METADATA:")):
		r.handleMetadataPhase(msg)
	case string(msg.Data) == "EOF":
		r.handleEOFPhase(msg)
	default:
		r.handleFileDataPhase(msg)
	}
}

// handleMetadataPhase processes metadata messages
func (r *ReceiverChannel) handleMetadataPhase(msg webrtc.DataChannelMessage) {
	metadata, err := r.processMetadata(msg.Data)
	if err != nil {
		log.Printf("Error handling metadata: %v", err)
		r.doneOnce.Do(func() { close(r.doneCh) })
		return
	}

	// Set up progress tracking with metadata
	r.fileMetadata = metadata

	// Send initial progress (non-blocking)
	select {
	case r.progressCh <- types.ProgressUpdate{
		NewBytes: 0,
		MetaData: metadata,
	}:
	default:
		log.Printf("Progress channel full, skipping metadata progress update")
	}

	// Prepare file for receiving with metadata
	finalPath, err := r.dataProcessor.PrepareFileForReceiving(r.destPath, metadata)
	if err != nil {
		log.Printf("Error preparing file for receiving: %v", err)
		r.doneOnce.Do(func() { close(r.doneCh) })
		return
	}

	log.Printf("Ready to receive file to: %s", finalPath)
}

// processMetadata extracts and decodes metadata from message
func (r *ReceiverChannel) processMetadata(msg []byte) (*types.FileMetadata, error) {
	metadataBytes := msg[9:] // Remove "METADATA:" prefix
	metadata, err := utils.DecodeJSON[types.FileMetadata](metadataBytes)
	if err != nil {
		return nil, fmt.Errorf("error decoding metadata: %w", err)
	}

	log.Printf("Received metadata: %s (size: %d bytes, type: %s)",
		metadata.Name, metadata.Size, metadata.MimeType)

	r.metadataReceived = true

	return &metadata, nil
}

// handleEOFPhase processes EOF messages and completes transfer
func (r *ReceiverChannel) handleEOFPhase(_ webrtc.DataChannelMessage) {
	totalBytes, err := r.dataProcessor.FinishReceiving()
	if err != nil {
		log.Printf("Error processing EOF signal: %v", err)
	}

	log.Printf("File transfer complete: %d bytes received", totalBytes)

	// Send final progress (non-blocking)
	update := types.ProgressUpdate{
		NewBytes: 0,
	}

	select {
	case r.progressCh <- update:
	default:
	}

	// Signal completion
	r.doneOnce.Do(func() { close(r.doneCh) })
}

// handleFileDataPhase processes file data messages
func (r *ReceiverChannel) handleFileDataPhase(msg webrtc.DataChannelMessage) {
	if !r.metadataReceived {
		log.Printf("Received file data before metadata, ignoring")
		return
	}

	// Write data using DataProcessor
	err := r.dataProcessor.WriteData(msg.Data)
	if err != nil {
		log.Printf("Error writing data: %v", err)
		return
	}

	// Send progress update (non-blocking)
	update := types.ProgressUpdate{
		NewBytes: uint64(len(msg.Data)),
	}

	select {
	case r.progressCh <- update:
	default:
		// Progress channel full, skip this update to avoid blocking data transfer
	}
}
