package signalling

import (
	"context"
	"fmt"
	"log"

	"yapfs/internal/config"
	"yapfs/pkg/utils"

	"github.com/pion/webrtc/v4"
)

// SignalingServer defines the interface for signaling storage operations
type SignalingServer interface {
	CreateSession(ctx context.Context, offer string) (sessionID string, err error)
	GetOffer(ctx context.Context, sessionID string) (offer string, err error)
	UpdateAnswer(ctx context.Context, sessionID, answer string) error
	WaitForAnswer(ctx context.Context, sessionID string) (answer string, err error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// SDPHandler defines the interface for WebRTC SDP operations
type SDPHandler interface {
	CreateOffer(peerConn *webrtc.PeerConnection) (*webrtc.SessionDescription, error)
	CreateAnswer(peerConn *webrtc.PeerConnection) (*webrtc.SessionDescription, error)
	WaitForICEGathering(ctx context.Context, peerConn *webrtc.PeerConnection) error
}

// SignalingService orchestrates the complete signaling flow using composition
type SignalingService struct {
	server SignalingServer
	sdp    SDPHandler
}

func NewSignalingService(server SignalingServer, sdp SDPHandler) *SignalingService {
	return &SignalingService{
		server: server,
		sdp:    sdp,
	}
}

func NewDefaultSignalingService(cfg *config.Config) (*SignalingService, error) {
	server, err := NewFirebaseClient(context.Background(), &cfg.Firebase)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase cilent: %w", err)
	}

	sdp := &WebRTCHandler{}

	return NewSignalingService(server, sdp), nil
}

func (s *SignalingService) StartSenderSignallingProcess(ctx context.Context, peerConn *webrtc.PeerConnection) (string, error) {
	// Create offer using SDP handler
	_, err := s.sdp.CreateOffer(peerConn)
	if err != nil {
		return "", fmt.Errorf("failed to create offer: %w", err)
	}

	// Wait for ICE gathering to complete
	err = s.sdp.WaitForICEGathering(ctx, peerConn)
	if err != nil {
		return "", fmt.Errorf("failed to wait for ICE gathering: %w", err)
	}

	// Get the final offer with ICE candidates
	finalOffer := peerConn.LocalDescription()
	if finalOffer == nil {
		return "", fmt.Errorf("local description is nil after ICE gathering")
	}

	// Encode offer SDP
	encodedOffer, err := utils.Encode(*finalOffer)
	if err != nil {
		return "", fmt.Errorf("failed to encode offer SDP: %w", err)
	}

	// Create session with offer using backend
	sessionID, err := s.server.CreateSession(ctx, encodedOffer)
	if err != nil {
		return "", fmt.Errorf("failed to create session with offer: %w", err)
	}

	log.Printf("Send this code to the receiver: %s\n", sessionID)

	// Wait for answer from remote peer
	answer, err := s.server.WaitForAnswer(ctx, sessionID)
	if err != nil {
		return sessionID, fmt.Errorf("failed to wait for answer: %w", err)
	}

	answerSD, err := utils.Decode[webrtc.SessionDescription](answer)
	if err != nil {
		return sessionID, fmt.Errorf("failed to decode answer SDP: %w", err)
	}

	err = peerConn.SetRemoteDescription(answerSD)
	if err != nil {
		return sessionID, fmt.Errorf("failed to set remote description: %w", err)
	}

	return sessionID, nil
}

// StartReceiverSignallingProcess orchestrates the complete receiver signaling flow
func (s *SignalingService) StartReceiverSignallingProcess(ctx context.Context, peerConn *webrtc.PeerConnection, sessionID string) error {
	// Get the offer from the session using backend
	encodedOffer, err := s.server.GetOffer(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get offer from session: %w", err)
	}

	// Decode the received offer
	offerSD, err := utils.Decode[webrtc.SessionDescription](encodedOffer)
	if err != nil {
		return fmt.Errorf("failed to decode offer SDP: %w", err)
	}

	// Set remote description
	err = peerConn.SetRemoteDescription(offerSD)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer using SDP handler
	_, err = s.sdp.CreateAnswer(peerConn)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Wait for ICE gathering to complete
	err = s.sdp.WaitForICEGathering(ctx, peerConn)
	if err != nil {
		return fmt.Errorf("failed to wait for ICE gathering: %w", err)
	}

	// Get the final answer with ICE candidates
	finalAnswer := peerConn.LocalDescription()
	if finalAnswer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Encode answer SDP
	encodedAnswer, err := utils.Encode(*finalAnswer)
	if err != nil {
		return fmt.Errorf("failed to encode answer SDP: %w", err)
	}

	// Upload answer using backend
	err = s.server.UpdateAnswer(ctx, sessionID, encodedAnswer)
	if err != nil {
		return fmt.Errorf("failed to upload answer: %w", err)
	}

	return nil
}

// ClearSession deletes a session by its ID
func (s *SignalingService) ClearSession(ctx context.Context, sessionID string) error {
	return s.server.DeleteSession(ctx, sessionID)
}
