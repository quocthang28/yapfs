package transport

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/pion/webrtc/v4"
)

// SignalingService handles SDP offer/answer exchange and encoding/decoding
type SignalingService struct{}

// NewSignalingService creates a new signaling service
func NewSignalingService() *SignalingService {
	return &SignalingService{}
}

// CreateOffer creates and sets an SDP offer for the peer connection
func (s *SignalingService) CreateOffer(peerConn *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
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
func (s *SignalingService) CreateAnswer(peerConn *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
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

// SetRemoteDescription sets the remote session description
func (s *SignalingService) SetRemoteDescription(peerConn *webrtc.PeerConnection, sd webrtc.SessionDescription) error {
	err := peerConn.SetRemoteDescription(sd)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}
	return nil
}

// WaitForICEGathering waits for ICE candidate gathering to complete
func (s *SignalingService) WaitForICEGathering(peerConn *webrtc.PeerConnection) <-chan struct{} {
	return webrtc.GatheringCompletePromise(peerConn)
}

// TODO: websocket trickle ICE
// func (s *SignalingService) StreamICECandidates(peerConn *webrtc.PeerConnection) {
// 	peerConn.OnICECandidate(func(i *webrtc.ICECandidate) {
// 		print(i)
// 	})
// }

// EncodeSessionDescription encodes a session description to base64
func (s *SignalingService) EncodeSessionDescription(sd webrtc.SessionDescription) (string, error) {
	bytes, err := json.Marshal(sd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session description: %w", err)
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// DecodeSessionDescription decodes a base64 encoded session description
func (s *SignalingService) DecodeSessionDescription(encoded string) (webrtc.SessionDescription, error) {
	var sd webrtc.SessionDescription

	bytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return sd, fmt.Errorf("failed to decode base64: %w", err)
	}

	err = json.Unmarshal(bytes, &sd)
	if err != nil {
		return sd, fmt.Errorf("failed to unmarshal session description: %w", err)
	}

	return sd, nil
}
