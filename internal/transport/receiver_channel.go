package transport

import (
	"log"
	"sync"

	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// ReceiverChannel manages data channel operations for receiving files
type ReceiverChannel struct {
	config      *config.Config
	dataChannel *webrtc.DataChannel
}

// NewReceiverChannel creates a new data channel receiver
func NewReceiverChannel(cfg *config.Config) *ReceiverChannel {
	return &ReceiverChannel{
		config: cfg,
	}
}

// SetupFileReceiver sets up handlers for receiving files and returns a completion channel
func (r *ReceiverChannel) SetupFileReceiver(peerConn *webrtc.PeerConnection, dataProcessor *processor.DataProcessor, destPath string) (<-chan struct{}, error) {
	doneCh := make(chan struct{})
	var doneOnce sync.Once // Ensure doneCh is closed only once

	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		r.dataChannel = dataChannel // Store the received data channel internally
		log.Printf("Received data channel: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

		r.dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

			// Prepare file for receiving using DataProcessor
			err := dataProcessor.PrepareFileForReceiving(destPath)
			if err != nil {
				log.Printf("Error preparing file for receiving: %v", err)
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			log.Printf("Ready to receive file to: %s", destPath)
		})

		r.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			if string(msg.Data) == "EOF" {
				// Finish receiving and get total bytes
				totalBytes, err := dataProcessor.FinishReceiving()
				if err != nil {
					log.Printf("Error finishing file reception: %v", err)
				} else {
					log.Printf("File transfer complete: %d bytes received", totalBytes)
				}

				// Signal completion
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			// Write data using DataProcessor
			err := dataProcessor.WriteData(msg.Data)
			if err != nil {
				log.Printf("Error writing data: %v", err)
				return
			}
		})

		r.dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
			// Clean up any open files
			dataProcessor.Close()
		})

		r.dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			// Clean up any open files
			dataProcessor.Close()
		})
	})

	return doneCh, nil
}
