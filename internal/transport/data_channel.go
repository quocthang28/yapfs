package transport

import (
	"context"

	"yapfs/internal/config"
	"yapfs/pkg/types"

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

// CreateFileSenderDataChannel creates a data channel configured for sending files and initializes everything needed for transfer
func (d *DataChannelService) CreateFileSenderDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection, label string, filePath string) error {
	return d.sender.CreateFileSenderDataChannel(ctx, peerConn, label, filePath)
}

// SendFile performs a blocking file transfer (call this after connection is established)
func (d *DataChannelService) SendFile() (<-chan types.ProgressUpdate, error) {
	return d.sender.SendFile()
}

// SetupFileReceiver sets up handlers for receiving files
func (d *DataChannelService) SetupFileReceiver(ctx context.Context, peerConn *webrtc.PeerConnection, destPath string) error {
	return d.receiver.SetupFileReceiver(ctx, peerConn, destPath)
}

// ReceiveFile performs a blocking file receive (call this after connection is established)
func (d *DataChannelService) ReceiveFile() (<-chan types.ProgressUpdate, error) {
	return d.receiver.ReceiveFile()
}
