package transport

import (
	"context"
	"fmt"

	"yapfs/internal/config"

	"github.com/pion/webrtc/v4"
)

// DataChannelService manages data channel operations and flow control
// This is a facade that composes sender and receiver channels
type DataChannelService struct {
	sender   *SenderChannel
	receiver *ReceiverChannel
}

// NewDataChannelService creates a new data channel service
func NewDataChannelService(cfg *config.Config) *DataChannelService {
	return &DataChannelService{
		sender:   NewSenderChannel(cfg),
		receiver: NewReceiverChannel(cfg),
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files
func (d *DataChannelService) CreateFileSenderDataChannel(peerConn *webrtc.PeerConnection, label string) error {
	return d.sender.CreateFileSenderDataChannel(peerConn, label)
}

// SetupFileSender configures file sending with progress reporting
func (d *DataChannelService) SetupFileSender(ctx context.Context, filePath string) (<-chan struct{}, <-chan ProgressUpdate, error) {
	return d.sender.SetupFileSender(ctx, filePath)
}

// SetupFileReceiver sets up handlers for receiving files and returns completion and progress channels
func (d *DataChannelService) SetupFileReceiver(peerConn *webrtc.PeerConnection, destPath string) (<-chan struct{}, <-chan ProgressUpdate, error) {
	return d.receiver.SetupFileReceiver(peerConn, destPath)
}

// Close cleans up the DataChannelService resources
func (d *DataChannelService) Close() error {
	var errs []error

	if err := d.sender.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := d.receiver.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing data channel service: %v", errs)
	}
	return nil
}

// GetCleanupFunc returns a cleanup function for partially written files
func (d *DataChannelService) GetCleanupFunc() func() error {
	return func() error {
		// Delegate to receiver channel's data processor for cleanup
		return d.receiver.CleanupPartialFile()
	}
}
