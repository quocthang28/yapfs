package transport

import (
	"bytes"
	"fmt"
	"log"
	"sync"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// ReceiverChannel manages data channel operations for receiving files
type ReceiverChannel struct {
	config           *config.Config
	dataChannel      *webrtc.DataChannel
	dataProcessor    *processor.DataProcessor
	metadataReceived bool // Track if metadata has been received

	// Progress tracking
	totalBytes     uint64
	bytesReceived  uint64
	startTime      time.Time
	lastUpdateTime time.Time
	fileMetadata   *processor.FileMetadata
}

// NewReceiverChannel creates a new data channel receiver
func NewReceiverChannel(cfg *config.Config) *ReceiverChannel {
	return &ReceiverChannel{
		config:           cfg,
		dataProcessor:    processor.NewDataProcessor(),
		metadataReceived: false,
	}
}

// SetupFileReceiver sets up handlers for receiving files and returns completion and progress channels
func (r *ReceiverChannel) SetupFileReceiver(peerConn *webrtc.PeerConnection, destPath string) (<-chan struct{}, <-chan ProgressUpdate, error) {
	doneCh := make(chan struct{})
	progressCh := make(chan ProgressUpdate, 5) // Buffer progress updates
	var doneOnce sync.Once                     // Ensure doneCh is closed only once
	var progressOnce sync.Once                 // Ensure progressCh is closed only once

	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		r.dataChannel = dataChannel // Store the received data channel internally
		log.Printf("Received data channel: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

		r.dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d. Waiting for metadata...", r.dataChannel.Label(), r.dataChannel.ID())
			// Initialize progress tracking
			r.startTime = time.Now()
			r.lastUpdateTime = r.startTime
		})

		r.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			r.handleMessage(msg, destPath, doneCh, progressCh, &doneOnce, &progressOnce)
		})

		r.dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
			// Clean up any open files
			r.dataProcessor.Close()
			// Close progress channel if not already closed
			progressOnce.Do(func() { close(progressCh) })
		})

		r.dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			// Clean up any open files. //TODO: clean up partially written file
			r.dataProcessor.Close()
			// Close progress channel if not already closed
			progressOnce.Do(func() { close(progressCh) })
		})
	})

	return doneCh, progressCh, nil
}

// Close cleans up the ReceiverChannel resources
func (r *ReceiverChannel) Close() error {
	if r.dataProcessor != nil {
		return r.dataProcessor.Close()
	}
	return nil
}

func (r *ReceiverChannel) processMetaData(msg []byte) (*processor.FileMetadata, error) {
	metadataBytes := msg[9:] // Remove "METADATA:" prefix
	metadata, err := r.dataProcessor.DecodeMetadata(metadataBytes)
	if err != nil {
		return nil, fmt.Errorf("error decoding metadata: %w", err)
	}

	log.Printf("Received metadata: %s (size: %d bytes, type: %s)",
		metadata.Name, metadata.Size, metadata.MimeType)

	r.metadataReceived = true

	return metadata, nil
}

// MessageHandlerContext contains shared context for message handlers
type MessageHandlerContext struct {
	destPath     string
	doneCh       chan struct{}
	progressCh   chan ProgressUpdate
	doneOnce     *sync.Once
	progressOnce *sync.Once
}

// handleMessage dispatches messages to appropriate handlers based on type
func (r *ReceiverChannel) handleMessage(msg webrtc.DataChannelMessage, destPath string, doneCh chan struct{}, progressCh chan ProgressUpdate, doneOnce *sync.Once, progressOnce *sync.Once) {
	ctx := &MessageHandlerContext{
		destPath:     destPath,
		doneCh:       doneCh,
		progressCh:   progressCh,
		doneOnce:     doneOnce,
		progressOnce: progressOnce,
	}

	// Determine message type and dispatch to appropriate handler
	switch {
	case !r.metadataReceived && bytes.HasPrefix(msg.Data, []byte("METADATA:")):
		r.handleMetadataMessage(msg, ctx)
	case string(msg.Data) == "EOF":
		r.handleEOFMessage(msg, ctx)
	default:
		r.handleFileDataMessage(msg, ctx)
	}
}

// handleMetadataMessage processes metadata messages
func (r *ReceiverChannel) handleMetadataMessage(msg webrtc.DataChannelMessage, ctx *MessageHandlerContext) {
	metadata, err := r.processMetaData(msg.Data)
	if err != nil {
		log.Printf("Error handling metadata: %v", err)
		ctx.doneOnce.Do(func() { close(ctx.doneCh) })
		return
	}

	// Set up progress tracking with file size
	r.totalBytes = uint64(metadata.Size)
	r.bytesReceived = 0
	r.fileMetadata = metadata

	// Send initial progress
	ctx.progressCh <- ProgressUpdate{
		BytesSent:   0,
		BytesTotal:  r.totalBytes,
		Percentage:  0.0,
		Throughput:  0.0,
		ElapsedTime: 0,
		MetaData:    *metadata,
	}

	// Prepare file for receiving with metadata
	finalPath, err := r.dataProcessor.PrepareFileForReceiving(ctx.destPath, metadata)
	if err != nil {
		log.Printf("Error preparing file for receiving: %v", err)
		ctx.doneOnce.Do(func() { close(ctx.doneCh) })
		return
	}

	log.Printf("Ready to receive file to: %s", finalPath)
}

// handleEOFMessage processes EOF messages
func (r *ReceiverChannel) handleEOFMessage(_ webrtc.DataChannelMessage, ctx *MessageHandlerContext) {
	// Finish receiving and get total bytes
	totalBytes, err := r.dataProcessor.FinishReceiving()
	if err != nil {
		log.Printf("Error processing EOF signal: %v", err)
	}

	log.Printf("File transfer complete: %d bytes received", totalBytes)

	// Send final progress
	elapsed := time.Since(r.startTime)
	avgThroughput := float64(r.bytesReceived) / elapsed.Seconds() / (1024 * 1024) // MB/s
	update := ProgressUpdate{
		BytesSent:   r.bytesReceived,
		BytesTotal:  r.totalBytes,
		Percentage:  100.0,
		Throughput:  avgThroughput,
		ElapsedTime: elapsed,
	}
	if r.fileMetadata != nil {
		update.MetaData = *r.fileMetadata
	}
	ctx.progressCh <- update

	// Close progress channel and signal completion
	ctx.progressOnce.Do(func() { close(ctx.progressCh) })
	ctx.doneOnce.Do(func() { close(ctx.doneCh) })
}

// handleFileDataMessage processes file data messages
func (r *ReceiverChannel) handleFileDataMessage(msg webrtc.DataChannelMessage, ctx *MessageHandlerContext) {
	// Handle file data (only after metadata is received)
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

	// Update progress tracking
	r.bytesReceived += uint64(len(msg.Data))

	// Send progress update every second or when significant progress is made
	now := time.Now()
	if now.Sub(r.lastUpdateTime) >= time.Second || r.bytesReceived == r.totalBytes {
		elapsed := now.Sub(r.startTime)
		percentage := float64(r.bytesReceived) / float64(r.totalBytes) * 100.0
		throughput := float64(r.bytesReceived) / elapsed.Seconds() / (1024 * 1024) // MB/s

		update := ProgressUpdate{
			BytesSent:   r.bytesReceived,
			BytesTotal:  r.totalBytes,
			Percentage:  percentage,
			Throughput:  throughput,
			ElapsedTime: elapsed,
		}
		if r.fileMetadata != nil {
			update.MetaData = *r.fileMetadata
		}
		ctx.progressCh <- update

		r.lastUpdateTime = now
	}
}
