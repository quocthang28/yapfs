package transport

import (
	"log"

	"yapfs/internal/config"
	"yapfs/internal/coordinator"
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
				close(doneCh)
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
				close(doneCh)
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

// SetupChannelHandlers sets up data channel handlers with channel communication
func (r *ReceiverChannel) SetupChannelHandlers(peerConn *webrtc.PeerConnection, channels *coordinator.ReceiverChannels, destPath string) error {
	// OnDataChannel sets an event handler which is invoked when a data channel message arrives from a remote peer.
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		r.dataChannel = dataChannel // Store the received data channel internally
		log.Printf("Received data channel: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())

		r.dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d", r.dataChannel.Label(), r.dataChannel.ID())
			log.Printf("Ready to receive file to: %s", destPath)
		})

		r.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			if string(msg.Data) == "EOF" {
				// Send EOF chunk through channel
				select {
				case channels.DataReceived <- processor.DataChunk{Data: nil, EOF: true}:
				case <-channels.Complete:
					return
				}
				return
			}

			// Send data chunk through channel
			select {
			case channels.DataReceived <- processor.DataChunk{Data: msg.Data, EOF: false}:
			case <-channels.Complete:
				return
			}
		})

		r.dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
		})

		r.dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			select {
			case channels.Error <- err:
			default:
			}
		})
	})

	return nil
}
