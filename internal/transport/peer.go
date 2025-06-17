package transport

import (
	"fmt"

	"yapfs/internal/config"

	"github.com/pion/webrtc/v4"
)

// CleanupFunc defines a function type for cleanup operations on connection failure
type CleanupFunc func() error

// ConnectionFailureError represents a connection failure with cleanup capability
type ConnectionFailureError struct {
	State   webrtc.PeerConnectionState
	Role    string
	Message string
}

func (e *ConnectionFailureError) Error() string {
	return fmt.Sprintf("connection failed in %s state for %s: %s", e.State.String(), e.Role, e.Message)
}

// PeerService manages WebRTC peer connection lifecycle
type PeerService struct {
	config        *config.Config
	failureChan   chan *ConnectionFailureError
	cleanupFunc   CleanupFunc
}

// NewPeerService creates a new peer service with the given configuration
func NewPeerService(cfg *config.Config) *PeerService {
	return &PeerService{
		config:      cfg,
		failureChan: make(chan *ConnectionFailureError, 1),
	}
}

// CreatePeerConnection creates a new peer connection with the given configuration
func (p *PeerService) CreatePeerConnection() (*webrtc.PeerConnection, error) {
	webrtcConfig := webrtc.Configuration{
		ICEServers: p.config.WebRTC.ICEServers,
	}

	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	return pc, nil
}

// SetupConnectionStateHandler configures connection state change handling
func (p *PeerService) SetupConnectionStateHandler(peerConn *webrtc.PeerConnection, role string, cleanup CleanupFunc) {
	p.cleanupFunc = cleanup
	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		p.handleConnectionStateChange(state, role)
	})
}

// GetFailureChannel returns a channel that receives connection failures
func (p *PeerService) GetFailureChannel() <-chan *ConnectionFailureError {
	return p.failureChan
}

// Close gracefully closes the peer connection
func (p *PeerService) Close(peerConn *webrtc.PeerConnection) error {
	if peerConn == nil {
		return nil
	}
	return peerConn.Close()
}

// handleConnectionStateChange centralized connection state handling
func (p *PeerService) handleConnectionStateChange(state webrtc.PeerConnectionState, role string) {
	fmt.Printf("Peer Connection State has changed: %s (%s)\n", state.String(), role)

	switch state {
	case webrtc.PeerConnectionStateFailed:
		fmt.Println("Peer Connection has gone to failed")
		// Send failure notification instead of exiting
		select {
		case p.failureChan <- &ConnectionFailureError{
			State:   state,
			Role:    role,
			Message: "peer connection failed",
		}:
		default:
			// Channel full, ignore (shouldn't happen with buffer size 1)
		}
	case webrtc.PeerConnectionStateClosed:
		fmt.Println("Peer Connection has gone to closed")
		// Send closure notification instead of exiting
		select {
		case p.failureChan <- &ConnectionFailureError{
			State:   state,
			Role:    role,
			Message: "peer connection closed",
		}:
		default:
			// Channel full, ignore
		}
	}
}

// PerformCleanup performs cleanup operations without exiting
func (p *PeerService) PerformCleanup() error {
	if p.cleanupFunc != nil {
		if err := p.cleanupFunc(); err != nil {
			fmt.Printf("Error during cleanup: %v\n", err)
			return err
		}
		fmt.Println("Cleanup completed successfully")
	}
	return nil
}
