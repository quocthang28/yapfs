package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// SenderChannel manages data channel operations for sending files
type SenderChannel struct {
	config        *config.Config
	dataChannel   *webrtc.DataChannel
	dataProcessor *processor.DataProcessor
}

// NewSenderChannel creates a new data channel sender
func NewSenderChannel(cfg *config.Config) *SenderChannel {
	return &SenderChannel{
		config:        cfg,
		dataProcessor: processor.NewDataProcessor(),
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files and stores it internally
func (s *SenderChannel) CreateFileSenderDataChannel(peerConn *webrtc.PeerConnection, label string) error {
	ordered := true //TODO: once data processor handle chunking this should be false

	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := peerConn.CreateDataChannel(label, options)
	if err != nil {
		return fmt.Errorf("failed to create file data channel: %w", err)
	}

	s.dataChannel = dataChannel
	return nil
}

// SetupFileSender configures file sending using the internal data channel
func (s *SenderChannel) SetupFileSender(ctx context.Context, filePath string) (<-chan struct{}, error) {
	if s.dataChannel == nil {
		return nil, fmt.Errorf("data channel not created, call CreateFileSenderDataChannel first")
	}
	sendMoreCh := make(chan struct{}, 3) // Buffer size 3 to handle multiple low threshold events
	doneCh := make(chan struct{})
	var doneOnce sync.Once // Ensure doneCh is closed only once

	// Prepare file for sending
	err := s.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare file for sending: %w", err)
	}

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	s.dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d. Sending metadata first...",
			s.dataChannel.Label(), s.dataChannel.ID())

		go func() {
			// First, send file metadata
			metadataBytes, err := s.dataProcessor.CreateFileMetadata(filePath)
			if err != nil {
				log.Printf("Error creating file metadata: %v", err)
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			// Send metadata with "METADATA:" prefix
			metadataMsg := append([]byte("METADATA:"), metadataBytes...)
			err = s.dataChannel.Send(metadataMsg)
			if err != nil {
				log.Printf("Error sending metadata: %v", err)
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			var totalBytesSent uint64

			// Start file transfer
			dataCh, errCh := s.dataProcessor.StartReadingFile(s.config.WebRTC.PacketSize)
			if dataCh == nil || errCh == nil {
				log.Printf("No file prepared for transfer")
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			// Process data chunks
			for {
				select {
				case chunk, ok := <-dataCh:
					if !ok {
						// Channel closed unexpectedly, TODO: clean up partially written file
						log.Printf("Data channel closed unexpectedly")
						doneOnce.Do(func() { close(doneCh) })
						return
					}

					if chunk.EOF {
						// Send EOF marker
						err := s.dataChannel.Send([]byte("EOF"))
						if err != nil {
							log.Printf("Error sending EOF: %v", err)
						} else {
							log.Printf("File transfer complete: %d bytes sent", totalBytesSent)
						}

						// Close the channel after sending EOF
						err = s.dataChannel.GracefulClose()
						if err != nil {
							log.Printf("Error closing channel: %v", err)
						}

						doneOnce.Do(func() { close(doneCh) })
						return
					}

					// Send data chunk
					err := s.dataChannel.Send(chunk.Data)
					if err != nil {
						log.Printf("Error sending data: %v", err)
						doneOnce.Do(func() { close(doneCh) })
						return
					}

					totalBytesSent += uint64(len(chunk.Data))

					// Flow control: wait if buffer is too full
					if s.dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
						select {
						case <-sendMoreCh:
							// Buffer drained, continue sending
						case <-ctx.Done():
							log.Printf("File transfer cancelled: %v", ctx.Err())
							doneOnce.Do(func() { close(doneCh) })
							return
						case <-time.After(30 * time.Second):
							log.Printf("Flow control timeout - WebRTC channel may be dead")
							doneOnce.Do(func() { close(doneCh) })
							return
						}
					}

				case err := <-errCh:
					log.Printf("Error during file transfer: %v", err)
					doneOnce.Do(func() { close(doneCh) })
					return
				case <-ctx.Done():
					log.Printf("File transfer cancelled: %v", ctx.Err())
					doneOnce.Do(func() { close(doneCh) })
					return
				}
			}
		}()
	})

	// Set up flow control
	s.dataChannel.SetBufferedAmountLowThreshold(s.config.WebRTC.BufferedAmountLowThreshold)
	s.dataChannel.OnBufferedAmountLow(func() {
		// Use non-blocking send with increased buffer to reduce dropped signals
		// If buffer is full, it means we already have pending signals
		select {
		case sendMoreCh <- struct{}{}:
			// Signal sent successfully
		default:
			// Buffer full - signal already pending, no need to add more
			log.Printf("Flow control: signal already pending, skipping")
		}
	})

	return doneCh, nil
}

// SetupFileSenderWithProgress configures file sending with progress reporting
func (s *SenderChannel) SetupFileSenderWithProgress(ctx context.Context, filePath string) (<-chan struct{}, <-chan processor.ProgressUpdate, error) {
	if s.dataChannel == nil {
		return nil, nil, fmt.Errorf("data channel not created, call CreateFileSenderDataChannel first")
	}
	sendMoreCh := make(chan struct{}, 3) // Buffer size 3 to handle multiple low threshold events
	doneCh := make(chan struct{})
	progressCh := make(chan processor.ProgressUpdate, 5) // Buffer progress updates
	var doneOnce sync.Once // Ensure doneCh is closed only once

	// Prepare file for sending
	err := s.dataProcessor.PrepareFileForSending(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare file for sending: %w", err)
	}

	// OnOpen sets an event handler which is invoked when the underlying data transport has been established (or re-established).
	s.dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d. Sending metadata first...",
			s.dataChannel.Label(), s.dataChannel.ID())

		go func() {
			defer close(progressCh) // Close progress channel when done
			
			// First, send file metadata
			metadataBytes, err := s.dataProcessor.CreateFileMetadata(filePath)
			if err != nil {
				log.Printf("Error creating file metadata: %v", err)
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			// Send metadata with "METADATA:" prefix
			metadataMsg := append([]byte("METADATA:"), metadataBytes...)
			err = s.dataChannel.Send(metadataMsg)
			if err != nil {
				log.Printf("Error sending metadata: %v", err)
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			// Start file transfer with progress
			dataCh, progressUpdateCh, errCh := s.dataProcessor.StartReadingFileWithProgress(s.config.WebRTC.PacketSize)
			if dataCh == nil || errCh == nil || progressUpdateCh == nil {
				log.Printf("No file prepared for transfer")
				doneOnce.Do(func() { close(doneCh) })
				return
			}

			// Forward progress updates
			go func() {
				for update := range progressUpdateCh {
					select {
					case progressCh <- update:
					case <-ctx.Done():
						return
					}
				}
			}()

			// Process data chunks
			for {
				select {
				case chunk, ok := <-dataCh:
					if !ok {
						// Channel closed unexpectedly, TODO: clean up partially written file
						log.Printf("Data channel closed unexpectedly")
						doneOnce.Do(func() { close(doneCh) })
						return
					}

					if chunk.EOF {
						// Send EOF marker
						err := s.dataChannel.Send([]byte("EOF"))
						if err != nil {
							log.Printf("Error sending EOF: %v", err)
						} else {
							log.Printf("File transfer complete")
						}

						// Close the channel after sending EOF
						err = s.dataChannel.GracefulClose()
						if err != nil {
							log.Printf("Error closing channel: %v", err)
						}

						doneOnce.Do(func() { close(doneCh) })
						return
					}

					// Send data chunk
					err := s.dataChannel.Send(chunk.Data)
					if err != nil {
						log.Printf("Error sending data: %v", err)
						doneOnce.Do(func() { close(doneCh) })
						return
					}

					// Flow control: wait if buffer is too full
					if s.dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
						select {
						case <-sendMoreCh:
							// Buffer drained, continue sending
						case <-ctx.Done():
							log.Printf("File transfer cancelled: %v", ctx.Err())
							doneOnce.Do(func() { close(doneCh) })
							return
						case <-time.After(30 * time.Second):
							log.Printf("Flow control timeout - WebRTC channel may be dead")
							doneOnce.Do(func() { close(doneCh) })
							return
						}
					}

				case err := <-errCh:
					log.Printf("Error during file transfer: %v", err)
					doneOnce.Do(func() { close(doneCh) })
					return
				case <-ctx.Done():
					log.Printf("File transfer cancelled: %v", ctx.Err())
					doneOnce.Do(func() { close(doneCh) })
					return
				}
			}
		}()
	})

	// Set up flow control
	s.dataChannel.SetBufferedAmountLowThreshold(s.config.WebRTC.BufferedAmountLowThreshold)
	s.dataChannel.OnBufferedAmountLow(func() {
		// Use non-blocking send with increased buffer to reduce dropped signals
		// If buffer is full, it means we already have pending signals
		select {
		case sendMoreCh <- struct{}{}:
			// Signal sent successfully
		default:
			// Buffer full - signal already pending, no need to add more
			log.Printf("Flow control: signal already pending, skipping")
		}
	})

	return doneCh, progressCh, nil
}

// Close cleans up the SenderChannel resources
func (s *SenderChannel) Close() error {
	if s.dataProcessor != nil {
		return s.dataProcessor.Close()
	}
	return nil
}
