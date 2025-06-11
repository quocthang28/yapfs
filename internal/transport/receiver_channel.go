package transport

import (
	"log"
	"os"

	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// ReceiverChannel manages data channel operations for receiving files
type ReceiverChannel struct {
	config *config.Config
}

// NewReceiverChannel creates a new data channel receiver
func NewReceiverChannel(cfg *config.Config) *ReceiverChannel {
	return &ReceiverChannel{
		config: cfg,
	}
}

// SetupFileReceiver sets up handlers for receiving files and returns a completion channel
func (r *ReceiverChannel) SetupFileReceiver(peerConn *webrtc.PeerConnection, dataProcessor *processor.DataProcessor, dstPath string) (<-chan struct{}, error) {
	doneCh := make(chan struct{})

	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Printf("Received data channel: %s-%d", dataChannel.Label(), dataChannel.ID())

		var fileWriter *os.File
		var totalBytesReceived uint64

		dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d", dataChannel.Label(), dataChannel.ID())

			// Create destination file
			var err error
			fileWriter, err = dataProcessor.CreateWriter(dstPath)
			if err != nil {
				log.Printf("Error creating destination file: %v", err)
				return
			}

			log.Printf("Ready to receive file to: %s", dstPath)
		})

		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			if fileWriter == nil {
				log.Printf("Received data but file not ready")
				return
			}

			if string(msg.Data) == "EOF" {
				log.Printf("File transfer complete: %d bytes received", totalBytesReceived)
				fileWriter.Close()
				// Signal completion
				close(doneCh)
				return
			}

			n, err := fileWriter.Write(msg.Data)
			if err != nil {
				log.Printf("Error writing to file: %v", err)
				return
			}
			
			totalBytesReceived += uint64(n)
		})

		dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
			if fileWriter != nil {
				fileWriter.Close()
			}
		})

		dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			if fileWriter != nil {
				fileWriter.Close()
			}
		})
	})

	return doneCh, nil
}