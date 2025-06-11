package transport

import (
	"yapfs/internal/config"
	"yapfs/internal/processor"

	"github.com/pion/webrtc/v4"
)

// DataChannelService manages data channel operations and flow control
// This is a facade (pattern) that composes sender and receiver services
type DataChannelService struct {
	sender   *SenderService
	receiver *ReceiverService
}

// NewDataChannelService creates a new data channel service
func NewDataChannelService(cfg *config.Config) *DataChannelService {
	return &DataChannelService{
		sender:   NewSenderService(cfg),
		receiver: NewReceiverService(cfg),
	}
}

// CreateFileSenderDataChannel creates a data channel configured for sending files
func (d *DataChannelService) CreateFileSenderDataChannel(pc *webrtc.PeerConnection, label string) (*webrtc.DataChannel, error) {
	return d.sender.CreateFileSenderDataChannel(pc, label)
}

// SetupFileSender configures file sending for a data channel
func (d *DataChannelService) SetupFileSender(dataChannel *webrtc.DataChannel, dataProcessor *processor.DataProcessor, filePath string) error {
	return d.sender.SetupFileSender(dataChannel, dataProcessor, filePath)
}

// SetupFileReceiver sets up handlers for receiving files and returns a completion channel
func (d *DataChannelService) SetupFileReceiver(pc *webrtc.PeerConnection, dataProcessor *processor.DataProcessor, dstPath string) (<-chan struct{}, error) {
	return d.receiver.SetupFileReceiver(pc, dataProcessor, dstPath)
}