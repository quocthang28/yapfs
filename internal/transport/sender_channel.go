package transport

import (
	"fmt"
	"io"
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

// SetupFileSender configures file sending for a data channel
func (s *SenderChannel) SetupFileSender(dataChannel *webrtc.DataChannel, dataProcessor *processor.DataProcessor, filePath string) error {
	sendMoreCh := make(chan struct{}, 1)

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d. Starting file transfer: %s",
			dataChannel.Label(), dataChannel.ID(), filePath)

		go func() {
			fileReader, err := dataProcessor.OpenReader(filePath)
			if err != nil {
				log.Printf("Error opening file: %v", err)
				return
			}
			defer fileReader.Close()

			stat, err := fileReader.Stat()
			if err != nil {
				log.Printf("Error getting file info: %v", err)
				return
			}
			fileSize := stat.Size()
			log.Printf("File size: %d bytes (%s)", fileSize, dataProcessor.FormatFileSize(fileSize))

			buffer := make([]byte, s.config.WebRTC.PacketSize)
			var totalBytesSent uint64

			for {
				n, err := fileReader.Read(buffer)
				if err == io.EOF {
					// Send EOF marker
					err = dataChannel.Send([]byte("EOF"))
					if err != nil {
						log.Printf("Error sending EOF: %v", err)
					} else {
						log.Printf("File transfer complete: %d bytes sent", totalBytesSent)
					}
					break
				}
				if err != nil {
					log.Printf("Error reading file: %v", err)
					break
				}

				data := buffer[:n]
				err = dataChannel.Send(data)
				if err != nil {
					log.Printf("Error sending data: %v", err)
					break
				}

				totalBytesSent += uint64(n)

				// Flow control: wait if buffer is too full
				if dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
					<-sendMoreCh
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

	return nil
}