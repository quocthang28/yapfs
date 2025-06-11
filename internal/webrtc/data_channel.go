// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync/atomic"
	"time"

	"github.com/pion/webrtc/v4"
	"yapfs/internal/config"
	"yapfs/internal/file"
)

// dataChannelService implements DataChannelService interface
type dataChannelService struct {
	config *config.Config
	throughputReporter ThroughputReporter
}

// NewDataChannelService creates a new data channel service
func NewDataChannelService(cfg *config.Config, reporter ThroughputReporter) DataChannelService {
	return &dataChannelService{
		config: cfg,
		throughputReporter: reporter,
	}
}

// CreateSenderDataChannel creates a data channel configured for sending with flow control
func (d *dataChannelService) CreateSenderDataChannel(pc *webrtc.PeerConnection, label string) (*webrtc.DataChannel, error) {
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

// SetupReceiverDataChannelHandler sets up handlers for incoming data channels
func (d *dataChannelService) SetupReceiverDataChannelHandler(pc *webrtc.PeerConnection) error {
	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		var totalBytesReceived uint64

		// Register channel opening handling
		dataChannel.OnOpen(func() {
			log.Printf("OnOpen: %s-%d. Start receiving data", dataChannel.Label(), dataChannel.ID())
			since := time.Now()

			// Start printing out the observed throughput
			ticker := time.NewTicker(time.Duration(d.config.WebRTC.ThroughputReportInterval) * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				bps := float64(atomic.LoadUint64(&totalBytesReceived)*8) / time.Since(since).Seconds()
				mbps := bps / 1024 / 1024
				
				if d.throughputReporter != nil {
					d.throughputReporter.OnThroughputUpdate(mbps)
				} else {
					log.Printf("Throughput: %.03f Mbps", mbps)
				}
			}
		})

		// Register the OnMessage to handle incoming messages
		dataChannel.OnMessage(func(dcMsg webrtc.DataChannelMessage) {
			n := len(dcMsg.Data)
			atomic.AddUint64(&totalBytesReceived, uint64(n))
		})
	})
	
	return nil
}

// StartSending begins the data sending process on the given channel
func (d *dataChannelService) StartSending(ctx context.Context, dc *webrtc.DataChannel) error {
	// This method can be used to start sending from outside the OnOpen callback
	// For now, the sending is automatically started in setupSenderFlowControl
	return nil
}

// setupSenderFlowControl configures flow control for a sending data channel
func (d *dataChannelService) setupSenderFlowControl(dataChannel *webrtc.DataChannel) {
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

// DefaultThroughputReporter provides a default implementation of ThroughputReporter
type DefaultThroughputReporter struct{}

// OnThroughputUpdate implements ThroughputReporter interface
func (d *DefaultThroughputReporter) OnThroughputUpdate(mbps float64) {
	log.Printf("Throughput: %.03f Mbps", mbps)
}

// CreateFileSenderDataChannel creates a data channel configured for sending files
func (d *dataChannelService) CreateFileSenderDataChannel(pc *webrtc.PeerConnection, label string, fileService file.FileService, filePath string) (*webrtc.DataChannel, error) {
	ordered := true
	
	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := pc.CreateDataChannel(label, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create file data channel: %w", err)
	}

	err = d.setupFileSender(dataChannel, fileService, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to setup file sender: %w", err)
	}

	return dataChannel, nil
}

// SetupFileReceiverDataChannelHandler sets up handlers for receiving files
func (d *dataChannelService) SetupFileReceiverDataChannelHandler(pc *webrtc.PeerConnection, fileService file.FileService, dstPath string) error {
	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Printf("Received data channel: %s-%d", dataChannel.Label(), dataChannel.ID())
		
		var fileWriter file.FileWriter
		var totalBytesReceived uint64

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
			since := time.Now()

			// Start reporting progress
			ticker := time.NewTicker(time.Duration(d.config.WebRTC.ThroughputReportInterval) * time.Millisecond)
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					bps := float64(atomic.LoadUint64(&totalBytesReceived)*8) / time.Since(since).Seconds()
					mbps := bps / 1024 / 1024
					
					if d.throughputReporter != nil {
						d.throughputReporter.OnThroughputUpdate(mbps)
					} else {
						log.Printf("Transfer: %.03f Mbps, %d bytes received", mbps, atomic.LoadUint64(&totalBytesReceived))
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
				log.Printf("File transfer complete: %d bytes received", atomic.LoadUint64(&totalBytesReceived))
				fileWriter.Close()
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
	
	return nil
}

// setupFileSender configures file sending for a data channel
func (d *dataChannelService) setupFileSender(dataChannel *webrtc.DataChannel, fileService file.FileService, filePath string) error {
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
						
						if d.throughputReporter != nil {
							d.throughputReporter.OnThroughputUpdate(mbps)
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

// SetupFileReceiverWithCompletion sets up handlers for receiving files and returns a completion channel
func (d *dataChannelService) SetupFileReceiverWithCompletion(pc *webrtc.PeerConnection, fileService file.FileService, dstPath string) (<-chan struct{}, error) {
	completionCh := make(chan struct{})
	
	pc.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		log.Printf("Received data channel: %s-%d", dataChannel.Label(), dataChannel.ID())
		
		var fileWriter file.FileWriter
		var totalBytesReceived uint64

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
			since := time.Now()

			// Start reporting progress
			ticker := time.NewTicker(time.Duration(d.config.WebRTC.ThroughputReportInterval) * time.Millisecond)
			defer ticker.Stop()
			go func() {
				for range ticker.C {
					bps := float64(atomic.LoadUint64(&totalBytesReceived)*8) / time.Since(since).Seconds()
					mbps := bps / 1024 / 1024
					
					if d.throughputReporter != nil {
						d.throughputReporter.OnThroughputUpdate(mbps)
					} else {
						log.Printf("Transfer: %.03f Mbps, %d bytes received", mbps, atomic.LoadUint64(&totalBytesReceived))
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
				log.Printf("File transfer complete: %d bytes received", atomic.LoadUint64(&totalBytesReceived))
				fileWriter.Close()
				// Signal completion
				close(completionCh)
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
	
	return completionCh, nil
}

