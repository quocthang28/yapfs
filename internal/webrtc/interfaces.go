package webrtc

import (
	"context"
	"github.com/pion/webrtc/v4"
	"yapfs/internal/file"
)

// PeerService manages WebRTC peer connection lifecycle
type PeerService interface {
	// CreatePeerConnection creates a new peer connection with the given configuration
	CreatePeerConnection(ctx context.Context) (*webrtc.PeerConnection, error)
	
	// SetupConnectionStateHandler configures connection state change handling
	SetupConnectionStateHandler(pc *webrtc.PeerConnection, role string)
	
	// Close gracefully closes the peer connection
	Close(pc *webrtc.PeerConnection) error
}

// DataChannelService manages data channel operations and flow control
type DataChannelService interface {
	// CreateSenderDataChannel creates a data channel configured for sending with flow control
	CreateSenderDataChannel(pc *webrtc.PeerConnection, label string) (*webrtc.DataChannel, error)
	
	// SetupReceiverDataChannelHandler sets up handlers for incoming data channels
	SetupReceiverDataChannelHandler(pc *webrtc.PeerConnection) error
	
	// StartSending begins the data sending process on the given channel
	StartSending(ctx context.Context, dc *webrtc.DataChannel) error
	
	// CreateFileSenderDataChannel creates a data channel configured for sending files
	CreateFileSenderDataChannel(pc *webrtc.PeerConnection, label string, fileService file.FileService, filePath string) (*webrtc.DataChannel, error)
	
	// SetupFileReceiverDataChannelHandler sets up handlers for receiving files
	SetupFileReceiverDataChannelHandler(pc *webrtc.PeerConnection, fileService file.FileService, dstPath string) error
	
	// SetupFileReceiverWithCompletion sets up handlers for receiving files and returns a completion channel
	SetupFileReceiverWithCompletion(pc *webrtc.PeerConnection, fileService file.FileService, dstPath string) (<-chan struct{}, error)
}

// SignalingService handles SDP offer/answer exchange and encoding/decoding
type SignalingService interface {
	// CreateOffer creates and sets an SDP offer for the peer connection
	CreateOffer(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error)
	
	// CreateAnswer creates and sets an SDP answer for the peer connection
	CreateAnswer(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error)
	
	// SetRemoteDescription sets the remote session description
	SetRemoteDescription(pc *webrtc.PeerConnection, sd webrtc.SessionDescription) error
	
	// WaitForICEGathering waits for ICE candidate gathering to complete
	WaitForICEGathering(ctx context.Context, pc *webrtc.PeerConnection) error
	
	// EncodeSessionDescription encodes a session description to base64
	EncodeSessionDescription(sd webrtc.SessionDescription) (string, error)
	
	// DecodeSessionDescription decodes a base64 encoded session description
	DecodeSessionDescription(encoded string) (webrtc.SessionDescription, error)
}

// ConnectionStateHandler defines callbacks for connection state changes
type ConnectionStateHandler interface {
	OnStateChange(state webrtc.PeerConnectionState, role string)
}

// DataChannelHandler defines callbacks for data channel events
type DataChannelHandler interface {
	OnOpen(label string, id uint16)
	OnMessage(data []byte)
	OnClose()
	OnError(err error)
}

// ThroughputReporter defines callbacks for throughput monitoring
type ThroughputReporter interface {
	OnThroughputUpdate(mbps float64)
}