package transport

import (
	"context"
	"fmt"
	"os"

	"yapfs/internal/config"

	"github.com/pion/webrtc/v4"
)

// PeerService manages WebRTC peer connection lifecycle
type PeerService struct {
	config       *config.Config
	stateHandler *DefaultConnectionStateHandler
}

// NewPeerService creates a new peer service with the given configuration
func NewPeerService(cfg *config.Config, stateHandler *DefaultConnectionStateHandler) *PeerService {
	return &PeerService{
		config:       cfg,
		stateHandler: stateHandler,
	}
}

// CreatePeerConnection creates a new peer connection with the given configuration
func (p *PeerService) CreatePeerConnection(ctx context.Context) (*webrtc.PeerConnection, error) {
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
func (p *PeerService) SetupConnectionStateHandler(peerConn *webrtc.PeerConnection, role string) {
	peerConn.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if p.stateHandler != nil {
			p.stateHandler.OnStateChange(state, role)
		} else {
			// Default behavior if no handler provided
			p.defaultStateHandler(state, role)
		}
	})
}

// Close gracefully closes the peer connection
func (p *PeerService) Close(peerConn *webrtc.PeerConnection) error {
	if peerConn == nil {
		return nil
	}
	return peerConn.Close()
}

// defaultStateHandler provides default connection state handling
func (p *PeerService) defaultStateHandler(state webrtc.PeerConnectionState, role string) {
	fmt.Printf("Peer Connection State has changed: %s (%s)\n", state.String(), role)

	if state == webrtc.PeerConnectionStateFailed {
		fmt.Println("Peer Connection has gone to failed exiting")
		os.Exit(0)
	}

	if state == webrtc.PeerConnectionStateClosed {
		fmt.Println("Peer Connection has gone to closed exiting")
		os.Exit(0)
	}
}

// DefaultConnectionStateHandler provides a default implementation of ConnectionStateHandler
type DefaultConnectionStateHandler struct{}

// OnStateChange implements ConnectionStateHandler interface
func (d *DefaultConnectionStateHandler) OnStateChange(state webrtc.PeerConnectionState, role string) {
	fmt.Printf("Peer Connection State has changed: %s (%s)\n", state.String(), role)

	if state == webrtc.PeerConnectionStateFailed {
		fmt.Println("Peer Connection has gone to failed exiting")
		os.Exit(0)
	}

	if state == webrtc.PeerConnectionStateClosed {
		fmt.Println("Peer Connection has gone to closed exiting")
		os.Exit(0)
	}
}
