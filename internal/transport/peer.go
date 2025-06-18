package transport

import (
	"context"
	"fmt"
	"log"

	"yapfs/internal/config"

	"github.com/pion/webrtc/v4"
)

// PeerConnection wraps webrtc.PeerConnection with state management
type PeerConnection struct {
	*webrtc.PeerConnection
	role    string
	closed  bool
	onError func(error)
	onConnected func()
	onClosed func()
}

// PeerService manages WebRTC peer connection lifecycle with centralized state management
type PeerService struct {
	config *config.Config
}

// NewPeerService creates a new peer service with the given configuration
func NewPeerService(cfg *config.Config) *PeerService {
	return &PeerService{
		config: cfg,
	}
}

// CreatePeerConnection creates a new peer connection with direct callback handling
// The callbacks are used directly in OnConnectionStateChange for state management
func (p *PeerService) CreatePeerConnection(ctx context.Context, role string, onError func(error), onConnected func(), onClosed func()) (*PeerConnection, error) {
	webrtcConfig := webrtc.Configuration{
		ICEServers: p.config.WebRTC.ICEServers,
	}

	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	wrappedPC := &PeerConnection{
		PeerConnection: pc,
		role:           role,
		closed:         false,
		onError:        onError,
		onConnected:    onConnected,
		onClosed:       onClosed,
	}

	// Set up state change handling with direct callbacks
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if wrappedPC.closed {
			return // Ignore state changes after close
		}

		log.Printf("Peer Connection State changed: %s (%s)", state.String(), role)

		switch state {
		case webrtc.PeerConnectionStateFailed:
			log.Printf("Peer Connection failed (%s)", role)
			if wrappedPC.onError != nil {
				wrappedPC.onError(fmt.Errorf("peer connection failed (%s)", role))
			}
		case webrtc.PeerConnectionStateConnected:
			log.Printf("Peer connection established successfully (%s)", role)
			if wrappedPC.onConnected != nil {
				wrappedPC.onConnected()
			}
		case webrtc.PeerConnectionStateClosed:
			log.Printf("Peer connection closed gracefully (%s)", role)
			wrappedPC.closed = true
			if wrappedPC.onClosed != nil {
				wrappedPC.onClosed()
			}
		}
	})

	return wrappedPC, nil
}

// IsConnected checks if the peer connection is in a connected state
func (pc *PeerConnection) IsConnected() bool {
	return !pc.closed && pc.ConnectionState() == webrtc.PeerConnectionStateConnected
}

// IsClosed checks if the peer connection has been closed
func (pc *PeerConnection) IsClosed() bool {
	return pc.closed
}

// Role returns the role of this peer connection
func (pc *PeerConnection) Role() string {
	return pc.role
}

// Close gracefully closes the peer connection
func (pc *PeerConnection) Close() error {
	if pc.closed {
		return nil // Already closed
	}

	pc.closed = true
	log.Printf("Closing peer connection (%s)", pc.role)
	return pc.PeerConnection.Close()
}

