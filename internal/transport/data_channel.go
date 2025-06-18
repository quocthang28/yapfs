package transport

import (
	"context"

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
func (d *DataChannelService) SetupFileSender(ctx context.Context, filePath string) (<-chan ProgressUpdate, error) {
	return d.sender.SetupFileSender(ctx, filePath)
}

// SendFile performs a blocking file transfer (call this after connection is established)
func (d *DataChannelService) SendFile() error {
	return d.sender.SendFile()
}

// SetupFileReceiver sets up handlers for receiving files and returns completion and progress channels
func (d *DataChannelService) SetupFileReceiver(peerConn *webrtc.PeerConnection, destPath string) (<-chan struct{}, <-chan ProgressUpdate, error) {
	return d.receiver.SetupFileReceiver(peerConn, destPath)
}
