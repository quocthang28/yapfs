package transport

import (
	"context"
	"fmt"
	"log"

	"yapfs/internal/config"

	"github.com/pion/webrtc/v4"
)

// ConnectionStateCallback defines the callback function for connection state changes
type ConnectionStateCallback func(state webrtc.PeerConnectionState, role string) error

// PeerConnection wraps webrtc.PeerConnection with state management
type PeerConnection struct {
	*webrtc.PeerConnection
	role         string
	stateHandler ConnectionStateCallback
	closed       bool
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

// CreatePeerConnection creates a new peer connection with centralized state management
func (p *PeerService) CreatePeerConnection(ctx context.Context, role string, stateHandler ConnectionStateCallback) (*PeerConnection, error) {
	webrtcConfig := webrtc.Configuration{
		ICEServers: p.config.WebRTC.ICEServers,
	}

	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	wrappedPC := &PeerConnection{
		PeerConnection: pc,
		role:          role,
		stateHandler:  stateHandler,
		closed:        false,
	}

	// Set up centralized state change handling
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if wrappedPC.closed {
			return // Ignore state changes after close
		}

		log.Printf("Peer Connection State changed: %s (%s)", state.String(), role)

		if stateHandler != nil {
			if err := stateHandler(state, role); err != nil {
				log.Printf("State handler error: %v", err)
			}
		} else {
			// Default behavior - just log critical states
			switch state {
			case webrtc.PeerConnectionStateFailed:
				log.Printf("Peer Connection failed (%s)", role)
			case webrtc.PeerConnectionStateClosed:
				log.Printf("Peer Connection closed (%s)", role)
				wrappedPC.closed = true
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

// DefaultStateHandler provides a sensible default state handler that doesn't exit the process
func DefaultStateHandler(state webrtc.PeerConnectionState, role string) error {
	switch state {
	case webrtc.PeerConnectionStateFailed:
		return fmt.Errorf("peer connection failed (%s)", role)
	case webrtc.PeerConnectionStateClosed:
		log.Printf("Peer connection closed gracefully (%s)", role)
		return nil
	case webrtc.PeerConnectionStateConnected:
		log.Printf("Peer connection established successfully (%s)", role)
		return nil
	default:
		// Other states are informational
		return nil
	}
}

// CreateDefaultStateHandler returns a state handler that propagates errors instead of exiting
func CreateDefaultStateHandler(onError func(error), onConnected func()) ConnectionStateCallback {
	return func(state webrtc.PeerConnectionState, role string) error {
		switch state {
		case webrtc.PeerConnectionStateFailed:
			err := fmt.Errorf("peer connection failed (%s)", role)
			if onError != nil {
				onError(err)
			}
			return err
		case webrtc.PeerConnectionStateConnected:
			log.Printf("Peer connection established successfully (%s)", role)
			if onConnected != nil {
				onConnected()
			}
			return nil
		case webrtc.PeerConnectionStateClosed:
			log.Printf("Peer connection closed gracefully (%s)", role)
			return nil
		default:
			return nil
		}
	}
}
