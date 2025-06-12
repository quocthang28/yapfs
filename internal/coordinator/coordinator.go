package coordinator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/pion/webrtc/v4"
	"yapfs/internal/config"
	"yapfs/internal/processor"
)

// FlowControlEvent represents flow control signaling
type FlowControlEvent struct {
	Type string // "pause", "resume"
}

// ProgressUpdate represents file transfer progress
type ProgressUpdate struct {
	BytesTransferred uint64
	TotalBytes       uint64
	Throughput       float64
	Percentage       float64
}

// SenderChannels defines communication channels for file sending
type SenderChannels struct {
	DataRequest  chan struct{}            // Transport requests data chunks
	DataResponse chan processor.DataChunk // Processor provides data chunks
	Progress     chan ProgressUpdate      // Progress reporting
	Error        chan error               // Error propagation
	Complete     chan struct{}            // Completion signaling
}

// ReceiverChannels defines communication channels for file receiving
type ReceiverChannels struct {
	DataReceived chan processor.DataChunk // Transport delivers received data
	Progress     chan ProgressUpdate      // Progress reporting
	Error        chan error               // Error propagation
	Complete     chan struct{}            // Completion signaling
}

// DataChannelServiceInterface defines the interface for data channel operations
type DataChannelServiceInterface interface {
	CreateFileSenderDataChannel(peerConn *webrtc.PeerConnection, label string) error
	SetupSenderChannelHandlers(channels *SenderChannels, totalBytes int64) error
	SendDataChunk(chunk processor.DataChunk) error
	SetupFileReceiverChannels(peerConn *webrtc.PeerConnection, channels *ReceiverChannels, destPath string) error
}

// FileTransferCoordinator mediates between transport and processing layers
type FileTransferCoordinator struct {
	config         *config.Config
	dataProcessor  *processor.DataProcessor
	dataChannelSvc DataChannelServiceInterface
}

// NewFileTransferCoordinator creates a new coordinator
func NewFileTransferCoordinator(cfg *config.Config, dataProcessor *processor.DataProcessor, dataChannelSvc DataChannelServiceInterface) *FileTransferCoordinator {
	return &FileTransferCoordinator{
		config:         cfg,
		dataProcessor:  dataProcessor,
		dataChannelSvc: dataChannelSvc,
	}
}

// CoordinateSender orchestrates the file sending process
func (c *FileTransferCoordinator) CoordinateSender(ctx context.Context, peerConn *webrtc.PeerConnection, filePath string) (<-chan struct{}, error) {
	// Create communication channels
	channels := &SenderChannels{
		DataRequest:  make(chan struct{}, 1),
		DataResponse: make(chan processor.DataChunk, 10), // Buffer for chunks
		Progress:     make(chan ProgressUpdate, 1),
		Error:        make(chan error, 1),
		Complete:     make(chan struct{}),
	}

	// Prepare file for sending in processor
	err := c.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare file for sending: %w", err)
	}

	// Set up data channel for sending
	err = c.dataChannelSvc.CreateFileSenderDataChannel(peerConn, "fileTransfer")
	if err != nil {
		return nil, fmt.Errorf("failed to create sender data channel: %w", err)
	}

	// Start processor goroutine
	go c.runSenderProcessor(ctx, channels)

	// Start transport goroutine
	go c.runSenderTransport(ctx, channels)

	// Start progress monitoring
	go c.monitorSenderProgress(ctx, channels)

	return channels.Complete, nil
}

// CoordinateReceiver orchestrates the file receiving process
func (c *FileTransferCoordinator) CoordinateReceiver(ctx context.Context, peerConn *webrtc.PeerConnection, destPath string) (<-chan struct{}, error) {
	// Create communication channels
	channels := &ReceiverChannels{
		DataReceived: make(chan processor.DataChunk, 10), // Buffer for received chunks
		Progress:     make(chan ProgressUpdate, 1),
		Error:        make(chan error, 1),
		Complete:     make(chan struct{}),
	}

	// Set up data channel for receiving
	err := c.dataChannelSvc.SetupFileReceiverChannels(peerConn, channels, destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to setup receiver data channel: %w", err)
	}

	// Start processor goroutine
	go c.runReceiverProcessor(ctx, channels, destPath)

	// Start progress monitoring
	go c.monitorReceiverProgress(ctx, channels)

	return channels.Complete, nil
}

// runSenderProcessor handles file reading and chunk production
func (c *FileTransferCoordinator) runSenderProcessor(ctx context.Context, channels *SenderChannels) {
	defer close(channels.DataResponse)

	for {
		select {
		case <-ctx.Done():
			channels.Error <- ctx.Err()
			return
		case <-channels.DataRequest:
			// Read next chunk from processor
			dataCh, errCh := c.dataProcessor.StartFileTransfer(c.config.WebRTC.PacketSize)

			// Forward chunks from processor to transport
			for {
				select {
				case <-ctx.Done():
					return
				case chunk, ok := <-dataCh:
					if !ok {
						return
					}
					select {
					case channels.DataResponse <- chunk:
						if chunk.EOF {
							return
						}
					case <-ctx.Done():
						return
					}
				case err := <-errCh:
					if err != nil {
						channels.Error <- err
						return
					}
				}
			}
		}
	}
}

// runSenderTransport handles WebRTC data channel sending
func (c *FileTransferCoordinator) runSenderTransport(ctx context.Context, channels *SenderChannels) {
	// Get file info for progress tracking
	fileInfo, err := c.dataProcessor.GetFileInfo()
	if err != nil {
		channels.Error <- fmt.Errorf("failed to get file info: %w", err)
		return
	}

	var totalBytesSent uint64
	startTime := time.Now()
	lastProgressTime := startTime

	// Set up data channel handlers through the service
	err = c.dataChannelSvc.SetupSenderChannelHandlers(channels, fileInfo.Size())
	if err != nil {
		channels.Error <- fmt.Errorf("failed to setup sender handlers: %w", err)
		return
	}

	ticker := time.NewTicker(time.Duration(c.config.WebRTC.ThroughputReportInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-channels.DataResponse:
			if !ok {
				return
			}

			// Send chunk through data channel service
			err := c.dataChannelSvc.SendDataChunk(chunk)
			if err != nil {
				channels.Error <- err
				return
			}

			if chunk.EOF {
				log.Printf("File transfer completed. Total bytes sent: %d", totalBytesSent)
				close(channels.Complete)
				return
			}

			totalBytesSent += uint64(len(chunk.Data))

		case <-ticker.C:
			// Send progress update
			now := time.Now()
			duration := now.Sub(lastProgressTime)
			if duration > 0 {
				throughput := float64(totalBytesSent) / duration.Seconds()
				percentage := float64(totalBytesSent) / float64(fileInfo.Size()) * 100

				select {
				case channels.Progress <- ProgressUpdate{
					BytesTransferred: totalBytesSent,
					TotalBytes:       uint64(fileInfo.Size()),
					Throughput:       throughput,
					Percentage:       percentage,
				}:
				default:
				}
			}
		}
	}
}

// runReceiverProcessor handles file writing from received chunks
func (c *FileTransferCoordinator) runReceiverProcessor(ctx context.Context, channels *ReceiverChannels, destPath string) {
	// Prepare file for receiving
	err := c.dataProcessor.PrepareFileForReceiving(destPath)
	if err != nil {
		channels.Error <- fmt.Errorf("failed to prepare file for receiving: %w", err)
		return
	}

	var totalBytesReceived uint64

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-channels.DataReceived:
			if !ok {
				return
			}

			if chunk.EOF {
				// Finish receiving
				finalBytes, err := c.dataProcessor.FinishReceiving()
				if err != nil {
					channels.Error <- fmt.Errorf("failed to finish receiving: %w", err)
					return
				}

				log.Printf("File transfer completed. Total bytes received: %d", finalBytes)
				close(channels.Complete)
				return
			}

			// Write chunk to file
			err := c.dataProcessor.WriteDataChunk(chunk.Data)
			if err != nil {
				channels.Error <- fmt.Errorf("failed to write data chunk: %w", err)
				return
			}

			totalBytesReceived += uint64(len(chunk.Data))
		}
	}
}

// monitorSenderProgress handles progress reporting for sender
func (c *FileTransferCoordinator) monitorSenderProgress(ctx context.Context, channels *SenderChannels) {
	for {
		select {
		case <-ctx.Done():
			return
		case progress := <-channels.Progress:
			log.Printf("Sending progress: %.1f%% (%d/%d bytes) at %.2f bytes/sec",
				progress.Percentage, progress.BytesTransferred, progress.TotalBytes, progress.Throughput)
		case err := <-channels.Error:
			log.Printf("Sender error: %v", err)
			return
		case <-channels.Complete:
			log.Printf("File sending completed successfully")
			return
		}
	}
}

// monitorReceiverProgress handles progress reporting for receiver
func (c *FileTransferCoordinator) monitorReceiverProgress(ctx context.Context, channels *ReceiverChannels) {
	for {
		select {
		case <-ctx.Done():
			return
		case progress := <-channels.Progress:
			log.Printf("Receiving progress: %.1f%% (%d bytes) at %.2f bytes/sec",
				progress.Percentage, progress.BytesTransferred, progress.Throughput)
		case err := <-channels.Error:
			log.Printf("Receiver error: %v", err)
			return
		case <-channels.Complete:
			log.Printf("File receiving completed successfully")
			return
		}
	}
}
