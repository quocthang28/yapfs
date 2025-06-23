package transport

import (
	"bytes"
	"context"
	"log"

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
	metadataReceived bool // Track if metadata has been received

	fileMetadata *types.FileMetadata
	readyCh      chan struct{} // Signals when data channel is open and ready
	destPath     string
}

// NewReceiverChannel creates a new data channel receiver
func NewReceiverChannel(cfg *config.Config) *ReceiverChannel {
	return &ReceiverChannel{
		config:           cfg,
		dataProcessor:    processor.NewDataProcessor(),
		metadataReceived: false,
		readyCh:          make(chan struct{}),
	}
}

// CreateFileReceiverDataChannel sets up data channel handlers for receiving files
func (r *ReceiverChannel) CreateFileReceiverDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection) error {
	r.ctx = ctx

	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		r.dataChannel = dataChannel
		log.Printf("Received data channel: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

		r.setupDataChannelHandlers()
	})

	return nil
}

// setupDataChannelHandlers configures data channel event handlers
func (r *ReceiverChannel) setupDataChannelHandlers() {
	r.dataChannel.OnOpen(func() {
		log.Printf("File transfer data channel opened: %s-%d. Waiting for metadata...", r.dataChannel.Label(), r.dataChannel.ID())
		close(r.readyCh) // Signal that channel is ready
	})

	r.dataChannel.OnClose(func() {
		log.Printf("File transfer data channel closed")
		r.dataProcessor.Close()
	})

	r.dataChannel.OnError(func(err error) {
		log.Printf("File transfer data channel error: %v", err)
		r.dataProcessor.Close()
	})
}

// ReceiveFile performs a non-blocking file receive, returns progress channel immediately
func (r *ReceiverChannel) ReceiveFile(destPath string) (<-chan types.ProgressUpdate, error) {
	progressCh := make(chan types.ProgressUpdate, 50)

	// Store destPath for use in message handlers
	r.destPath = destPath

	r.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		r.handleMessage(msg, progressCh)
	})

	// Wait for data channel to be ready
	select {
	case <-r.readyCh:
		log.Printf("Data channel ready, waiting for file transfer")
	case <-r.ctx.Done():
		log.Printf("Cancelled while waiting for data channel: %v", r.ctx.Err())
		return nil, r.ctx.Err()
	}

	return progressCh, nil
}

// handleMessage processes incoming WebRTC messages directly
func (r *ReceiverChannel) handleMessage(msg webrtc.DataChannelMessage, progressCh chan types.ProgressUpdate) {
	switch {
	case !r.metadataReceived && bytes.HasPrefix(msg.Data, []byte("METADATA:")):
		r.handleMetadataMessage(msg, progressCh)
	case string(msg.Data) == "EOF":
		r.handleEOFMessage()
	default:
		r.handleFileDataMessage(msg, progressCh)
	}
}

// handleMetadataMessage processes metadata messages
func (r *ReceiverChannel) handleMetadataMessage(msg webrtc.DataChannelMessage, progressCh chan types.ProgressUpdate) {
	metadataBytes := msg.Data[9:] // Remove "METADATA:" prefix
	metadata, err := utils.DecodeJSON[types.FileMetadata](metadataBytes)
	if err != nil {
		log.Printf("Error handling metadata: %v", err)
		return
	}

	log.Printf("Received metadata: %s (size: %d bytes, type: %s)",
		metadata.Name, metadata.Size, metadata.MimeType)

	r.metadataReceived = true
	r.fileMetadata = &metadata

	// Send initial progress (non-blocking)
	select {
	case progressCh <- types.ProgressUpdate{
		NewBytes: 0,
		MetaData: &metadata,
	}:
	default:
		log.Printf("Progress channel full, skipping metadata progress update")
	}

	// Prepare file for receiving
	finalPath, err := r.dataProcessor.PrepareFileForReceiving(r.destPath, &metadata)
	if err != nil {
		log.Printf("Error preparing file for receiving: %v", err)
		return
	}

	log.Printf("Ready to receive file to: %s", finalPath)
}

// handleFileDataMessage processes file data messages
func (r *ReceiverChannel) handleFileDataMessage(msg webrtc.DataChannelMessage, progressCh chan types.ProgressUpdate) {
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
	case progressCh <- update:
	default:
		// Progress channel full, skip this update to avoid blocking data transfer
	}
}

// handleEOFMessage processes EOF messages
func (r *ReceiverChannel) handleEOFMessage() {
	// Finish receiving and get total bytes
	totalBytes, err := r.dataProcessor.FinishReceiving()
	if err != nil {
		log.Printf("Error processing EOF signal: %v", err)
	}

	log.Printf("File transfer complete: %d bytes received", totalBytes)
}
