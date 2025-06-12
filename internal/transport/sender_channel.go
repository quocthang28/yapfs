package transport

import (
	"fmt"
	"log"

	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// SenderChannel manages data channel operations for sending files
type SenderChannel struct {
	config *config.Config
}

// NewSenderChannel creates a new data channel sender
func NewSenderChannel(cfg *config.Config) *SenderChannel {
	return &SenderChannel{
		config: cfg,
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files
func (s *SenderChannel) CreateFileSenderDataChannel(peerConn *webrtc.PeerConnection, label string) (*webrtc.DataChannel, error) {
	ordered := true //TODO: once data processor handle chunking this should be false

	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := peerConn.CreateDataChannel(label, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create file data channel: %w", err)
	}

	return dataChannel, nil
}

// SetupFileSender configures file sending for a data channel using prepared data processor
func (s *SenderChannel) SetupFileSender(dataChannel *webrtc.DataChannel, dataProcessor *processor.DataProcessor) (<-chan struct{}, error) {
	sendMoreCh := make(chan struct{}, 1)
	doneCh := make(chan struct{})

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	dataChannel.OnOpen(func() {
		fileInfo, err := dataProcessor.GetFileInfo()
		if err != nil {
			log.Printf("Error getting file info: %v", err)
			close(doneCh)
			return
		}
		
		log.Printf("File data channel opened: %s-%d. Starting file transfer, file size: %d bytes",
			dataChannel.Label(), dataChannel.ID(), fileInfo.Size())

		go func() {
			var totalBytesSent uint64

			// Start file transfer
			dataCh, errCh := dataProcessor.StartFileTransfer(s.config.WebRTC.PacketSize)
			if dataCh == nil || errCh == nil {
				log.Printf("No file prepared for transfer")
				close(doneCh)
				return
			}

			// Process data chunks
			for {
				select {
				case chunk, ok := <-dataCh:
					if !ok {
						// Channel closed unexpectedly
						log.Printf("Data channel closed unexpectedly")
						close(doneCh)
						return
					}

					if chunk.EOF {
						// Send EOF marker
						err := dataChannel.Send([]byte("EOF"))
						if err != nil {
							log.Printf("Error sending EOF: %v", err)
						} else {
							log.Printf("File transfer complete: %d bytes sent", totalBytesSent)
						}
						close(doneCh)
						return
					}

					// Send data chunk
					err := dataChannel.Send(chunk.Data)
					if err != nil {
						log.Printf("Error sending data: %v", err)
						close(doneCh)
						return
					}

					totalBytesSent += uint64(len(chunk.Data))

					// Flow control: wait if buffer is too full
					if dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
						<-sendMoreCh
					}

				case err := <-errCh:
					log.Printf("Error during file transfer: %v", err)
					close(doneCh)
					return
				}
			}
		}()
	})

	// Set up flow control
	dataChannel.SetBufferedAmountLowThreshold(s.config.WebRTC.BufferedAmountLowThreshold)
	dataChannel.OnBufferedAmountLow(func() {
		select {
		case sendMoreCh <- struct{}{}:
		default:
		}
	})

	return doneCh, nil
}
