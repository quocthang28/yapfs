package transport

import (
	"yapfs/internal/config"
	"yapfs/internal/coordinator"
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

// SetupSenderChannelHandlers sets up data channel handlers for sending with channel communication
func (d *DataChannelService) SetupSenderChannelHandlers(channels *coordinator.SenderChannels, totalBytes int64) error {
	return d.sender.SetupChannelHandlers(channels, totalBytes)
}

// SendDataChunk sends a data chunk through the data channel
func (d *DataChannelService) SendDataChunk(chunk processor.DataChunk) error {
	return d.sender.SendDataChunk(chunk)
}

// SetupFileReceiverChannels sets up handlers for receiving files with channel communication
func (d *DataChannelService) SetupFileReceiverChannels(peerConn *webrtc.PeerConnection, channels *coordinator.ReceiverChannels, destPath string) error {
	return d.receiver.SetupChannelHandlers(peerConn, channels, destPath)
}
