package webrtc

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/file"

	"github.com/pion/webrtc/v4"
)

// ProgressUpdate contains progress information for file transfers
type ProgressUpdate struct {
	Progress   float64 // Percentage complete (0-100)
	Throughput float64 // Throughput in Mbps
	BytesSent  int64   // Bytes sent so far
	BytesTotal int64   // Total bytes to send
}

// CompletionUpdate contains completion information for file transfers
type CompletionUpdate struct {
	Message string // Completion message
	Error   error  // Error if transfer failed
}

// DataChannelService manages data channel operations and flow control
type DataChannelService struct {
	config *config.Config
}

// NewDataChannelService creates a new data channel service
func NewDataChannelService(cfg *config.Config) *DataChannelService {
	return &DataChannelService{
		config: cfg,
	}
}

// CreateSenderDataChannel creates a data channel configured for sending with flow control
func (d *DataChannelService) CreateSenderDataChannel(pc *webrtc.PeerConnection, label string) (*webrtc.DataChannel, error) {
	ordered := false
	maxRetransmits := uint16(0)

	options := &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	}

	dataChannel, err := pc.CreateDataChannel(label, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create data channel: %w", err)
	}

	d.setupSenderFlowControl(dataChannel)
	return dataChannel, nil
}

// StartSending begins the data sending process on the given channel
func (d *DataChannelService) StartSending(ctx context.Context, dc *webrtc.DataChannel) error {
	// This method can be used to start sending from outside the OnOpen callback
	// For now, the sending is automatically started in setupSenderFlowControl
	return nil
}

// setupSenderFlowControl configures flow control for a sending data channel
func (d *DataChannelService) setupSenderFlowControl(dataChannel *webrtc.DataChannel) {
	buf := make([]byte, d.config.WebRTC.PacketSize)
	sendMoreCh := make(chan struct{}, 1)

	// Register channel opening handling
	dataChannel.OnOpen(func() {
		log.Printf(
			"OnOpen: %s-%d. Start sending a series of %d-byte packets as fast as it can\n",
			dataChannel.Label(), dataChannel.ID(), d.config.WebRTC.PacketSize,
		)

		for {
			err := dataChannel.Send(buf)
			if err != nil {
				log.Printf("Error sending data: %v", err)
				return
			}

			if dataChannel.BufferedAmount() > d.config.WebRTC.MaxBufferedAmount {
				// Wait until the bufferedAmount becomes lower than the threshold
				<-sendMoreCh
			}
		}
	})

	// Set bufferedAmountLowThreshold so that we can get notified when we can send more
	dataChannel.SetBufferedAmountLowThreshold(d.config.WebRTC.BufferedAmountLowThreshold)

	// This callback is made when the current bufferedAmount becomes lower than the threshold
	dataChannel.OnBufferedAmountLow(func() {
		// Make sure to not block this channel or perform long running operations in this callback
		// This callback is executed by pion/sctp. If this callback is blocking it will stop operations
		select {
		case sendMoreCh <- struct{}{}:
		default:
		}
	})
}

// CreateFileSenderDataChannel creates a data channel configured for sending files
func (d *DataChannelService) CreateFileSenderDataChannel(pc *webrtc.PeerConnection, label string) (*webrtc.DataChannel, error) {
	ordered := true

	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := pc.CreateDataChannel(label, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create file data channel: %w", err)
	}

	return dataChannel, nil
}

// setupFileSender configures file sending for a data channel
func (d *DataChannelService) SetupFileSender(dataChannel *webrtc.DataChannel, fileService *file.FileService, filePath string, progressCh chan<- ProgressUpdate, completionUpdateCh chan<- CompletionUpdate) error {
	sendMoreCh := make(chan struct{}, 1)

	dataChannel.OnOpen(func() {
		log.Printf("File data channel opened: %s-%d. Starting file transfer: %s",
			dataChannel.Label(), dataChannel.ID(), filePath)

		go func() {
			fileReader, err := fileService.OpenReader(filePath)
			if err != nil {
				log.Printf("Error opening file: %v", err)
				return
			}
			defer fileReader.Close()

			fileSize := fileReader.Size()
			log.Printf("File size: %d bytes (%s)", fileSize, fileService.FormatFileSize(fileSize))

			buffer := make([]byte, d.config.WebRTC.PacketSize)
			var totalBytesSent uint64
			startTime := time.Now()

			// Start progress reporting
			ticker := time.NewTicker(time.Duration(d.config.WebRTC.ThroughputReportInterval) * time.Millisecond)
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					sent := atomic.LoadUint64(&totalBytesSent)
					elapsed := time.Since(startTime).Seconds()
					if elapsed > 0 {
						bps := float64(sent*8) / elapsed
						mbps := bps / 1024 / 1024
						progress := float64(sent) / float64(fileSize) * 100

						if progressCh != nil {
							select {
							case progressCh <- ProgressUpdate{
								Progress:   progress,
								Throughput: mbps,
								BytesSent:  int64(sent),
								BytesTotal: fileSize,
							}:
							default:
								// Channel is full, skip this update
							}
						} else {
							log.Printf("Sending: %.03f Mbps, %.1f%% complete (%s/%s)",
								mbps, progress, fileService.FormatFileSize(int64(sent)), fileService.FormatFileSize(fileSize))
						}
					}
				}
			}()

			for {
				n, err := fileReader.Read(buffer)
				if err == io.EOF {
					// Send EOF marker
					err = dataChannel.Send([]byte("EOF"))
					if err != nil {
						log.Printf("Error sending EOF: %v", err)
					} else {
						message := fmt.Sprintf("File transfer complete: %d bytes sent", totalBytesSent)
						if completionUpdateCh != nil {
							select {
							case completionUpdateCh <- CompletionUpdate{Message: message}:
							default:
								// Channel is full or closed
							}
						} else {
							log.Printf("%s", message)
						}
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

				atomic.AddUint64(&totalBytesSent, uint64(n))

				// Flow control: wait if buffer is too full
				if dataChannel.BufferedAmount() > d.config.WebRTC.MaxBufferedAmount {
					<-sendMoreCh
				}
			}
		}()
	})

	// Set up flow control
	dataChannel.SetBufferedAmountLowThreshold(d.config.WebRTC.BufferedAmountLowThreshold)
	dataChannel.OnBufferedAmountLow(func() {
		select {
		case sendMoreCh <- struct{}{}:
		default:
		}
	})

	return nil
}

// SetupFileReceiver sets up handlers for receiving files with progress reporting and returns a completion channel
func (d *DataChannelService) SetupFileReceiver(pc *webrtc.PeerConnection, fileService *file.FileService, dstPath string, progressCh chan<- ProgressUpdate, completionCh chan<- CompletionUpdate) (<-chan struct{}, error) {
	doneCh := make(chan struct{})

	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Printf("Received data channel: %s-%d", dataChannel.Label(), dataChannel.ID())

		var fileWriter *file.FileWriter
		var totalBytesReceived uint64
		var startTime time.Time
		var progressTicker *time.Ticker

		dataChannel.OnOpen(func() {
			log.Printf("File transfer data channel opened: %s-%d", dataChannel.Label(), dataChannel.ID())

			// Create destination file
			var err error
			fileWriter, err = fileService.CreateWriter(dstPath)
			if err != nil {
				log.Printf("Error creating destination file: %v", err)
				return
			}

			log.Printf("Ready to receive file to: %s", dstPath)
			startTime = time.Now()

			// Start progress reporting ticker
			progressTicker = time.NewTicker(time.Duration(d.config.WebRTC.ThroughputReportInterval) * time.Millisecond)
			go func() {
				for range progressTicker.C {
					received := atomic.LoadUint64(&totalBytesReceived)
					elapsed := time.Since(startTime).Seconds()
					if elapsed > 0 && received > 0 {
						bps := float64(received*8) / elapsed
						mbps := bps / 1024 / 1024

						if progressCh != nil {
							select {
							case progressCh <- ProgressUpdate{
								Progress:   0, // We don't know total size ahead of time for receiver
								Throughput: mbps,
								BytesSent:  int64(received), // Using BytesSent for consistency, but it's actually received
								BytesTotal: 0,               // Unknown until transfer completes
							}:
							default:
								// Channel is full, skip this update
							}
						} else {
							log.Printf("Receiving: %.03f Mbps, %s received",
								mbps, fileService.FormatFileSize(int64(received)))
						}
					}
				}
			}()
		})

		dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
			if fileWriter == nil {
				log.Printf("Received data but file not ready")
				return
			}

			if string(msg.Data) == "EOF" {
				// Stop progress reporting
				if progressTicker != nil {
					progressTicker.Stop()
				}

				totalReceived := atomic.LoadUint64(&totalBytesReceived)
				message := fmt.Sprintf("File transfer complete: %d bytes received", totalReceived)

				// Send final progress update with 100% completion
				if progressCh != nil {
					select {
					case progressCh <- ProgressUpdate{
						Progress:   100.0,
						Throughput: 0,
						BytesSent:  int64(totalReceived),
						BytesTotal: int64(totalReceived),
					}:
					default:
						// Channel is full or closed
					}
				}

				if completionCh != nil {
					select {
					case completionCh <- CompletionUpdate{Message: message}:
					default:
						// Channel is full or closed
					}
				} else {
					log.Printf("%s", message)
				}
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
			atomic.AddUint64(&totalBytesReceived, uint64(n))
		})

		dataChannel.OnClose(func() {
			log.Printf("File transfer data channel closed")
			if progressTicker != nil {
				progressTicker.Stop()
			}
			if fileWriter != nil {
				fileWriter.Close()
			}
		})

		dataChannel.OnError(func(err error) {
			log.Printf("File transfer data channel error: %v", err)
			if progressTicker != nil {
				progressTicker.Stop()
			}
			if fileWriter != nil {
				fileWriter.Close()
			}
		})
	})

	return doneCh, nil
}
