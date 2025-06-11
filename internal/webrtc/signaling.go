// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/pion/webrtc/v4"
)

// signalingService implements SignalingService interface
type signalingService struct{}

// NewSignalingService creates a new signaling service
func NewSignalingService() SignalingService {
	return &signalingService{}
}

// CreateOffer creates and sets an SDP offer for the peer connection
func (s *signalingService) CreateOffer(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create offer: %w", err)
	}

	err = pc.SetLocalDescription(offer)
	if err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	return &offer, nil
}

// CreateAnswer creates and sets an SDP answer for the peer connection
func (s *signalingService) CreateAnswer(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create answer: %w", err)
	}

	err = pc.SetLocalDescription(answer)
	if err != nil {
		return nil, fmt.Errorf("failed to set local description: %w", err)
	}

	return &answer, nil
}

// SetRemoteDescription sets the remote session description
func (s *signalingService) SetRemoteDescription(pc *webrtc.PeerConnection, sd webrtc.SessionDescription) error {
	err := pc.SetRemoteDescription(sd)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}
	return nil
}

// WaitForICEGathering waits for ICE candidate gathering to complete
func (s *signalingService) WaitForICEGathering(ctx context.Context, pc *webrtc.PeerConnection) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-webrtc.GatheringCompletePromise(pc):
		return nil
	}
}

// EncodeSessionDescription encodes a session description to base64
func (s *signalingService) EncodeSessionDescription(sd webrtc.SessionDescription) (string, error) {
	bytes, err := json.Marshal(sd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session description: %w", err)
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// DecodeSessionDescription decodes a base64 encoded session description
func (s *signalingService) DecodeSessionDescription(encoded string) (webrtc.SessionDescription, error) {
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