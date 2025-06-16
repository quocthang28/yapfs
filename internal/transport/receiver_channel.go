package transport

import (
	"bytes"
	"fmt"
	"log"
	"sync"

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
}

// NewReceiverChannel creates a new data channel receiver
func NewReceiverChannel(cfg *config.Config) *ReceiverChannel {
	return &ReceiverChannel{
		config:           cfg,
		dataProcessor:    processor.NewDataProcessor(),
		metadataReceived: false,
	}
}

// SetupFileReceiver sets up handlers for receiving files and returns a completion channel
func (r *ReceiverChannel) SetupFileReceiver(peerConn *webrtc.PeerConnection, destPath string) (<-chan struct{}, error) {
	doneCh := make(chan struct{})
	var doneOnce sync.Once // Ensure doneCh is closed only once

	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		r.dataChannel = dataChannel // Store the received data channel internally
		log.Printf("Received data channel: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

		r.dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d. Waiting for metadata...", r.dataChannel.Label(), r.dataChannel.ID())
		})

		r.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			// Handle metadata
			if !r.metadataReceived && bytes.HasPrefix(msg.Data, []byte("METADATA:")) {
				metadata, err := r.processMetaData(msg.Data)
				if err != nil {
					log.Printf("Error handling metadata: %v", err)
					doneOnce.Do(func() { close(doneCh) })
					return
				}

				// Prepare file for receiving with metadata
				finalPath, err := r.dataProcessor.PrepareFileForReceiving(destPath, metadata)
				if err != nil {
					log.Printf("Error preparing file for receiving: %v", err)
					doneOnce.Do(func() { close(doneCh) })
					return
				}

				log.Printf("Ready to receive file to: %s", finalPath)
				return
			}

			// Handle EOF
			if string(msg.Data) == "EOF" {
				// Finish receiving and get total bytes
				err := r.processEOFSignal()
				if err != nil {
					log.Printf("Error processing EOF signal: %v", err)
				}

				// Signal completion
				doneOnce.Do(func() { close(doneCh) })
				return
			}

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
		})

		r.dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
			// Clean up any open files
			r.dataProcessor.Close()
		})

		r.dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			// Clean up any open files
			r.dataProcessor.Close()
		})
	})

	return doneCh, nil
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

func (r *ReceiverChannel) processEOFSignal() error {
	totalBytes, err := r.dataProcessor.FinishReceiving()
	if err != nil {
		return fmt.Errorf("error finishing file reception: %w", err)
	}

	log.Printf("File transfer complete: %d bytes received", totalBytes)

	return nil
}
