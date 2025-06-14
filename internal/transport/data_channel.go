package transport

import (
	"context"

	"yapfs/internal/config"
	"yapfs/internal/processor"

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

// SetupFileSender configures file sending using prepared data processor
func (d *DataChannelService) SetupFileSender(ctx context.Context, dataProcessor *processor.DataProcessor) (<-chan struct{}, error) {
	return d.sender.SetupFileSender(ctx, dataProcessor)
}

// SetupFileReceiver sets up handlers for receiving files and returns a completion channel
func (d *DataChannelService) SetupFileReceiver(peerConn *webrtc.PeerConnection, dataProcessor *processor.DataProcessor, destPath string) (<-chan struct{}, error) {
	return d.receiver.SetupFileReceiver(peerConn, dataProcessor, destPath)
}
