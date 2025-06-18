package signalling

import (
	"context"
	"fmt"

	"github.com/pion/webrtc/v4"
)

// WebRTCHandler implements SDPHandler for WebRTC operations
type WebRTCHandler struct{}

// CreateOffer creates and sets an SDP offer for the peer connection
func (h *WebRTCHandler) CreateOffer(peerConn *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	offer, err := peerConn.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	err = peerConn.SetLocalDescription(offer)
	if err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	return &offer, nil
}

// CreateAnswer creates and sets an SDP answer for the peer connection
func (h *WebRTCHandler) CreateAnswer(peerConn *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	answer, err := peerConn.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	err = peerConn.SetLocalDescription(answer)
	if err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	return &answer, nil
}

// WaitForICEGathering waits for ICE gathering to complete
func (h *WebRTCHandler) WaitForICEGathering(ctx context.Context, peerConn *webrtc.PeerConnection) error {
	select {
	case <-webrtc.GatheringCompletePromise(peerConn):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
