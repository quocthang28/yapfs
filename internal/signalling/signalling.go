package signalling

import (
	"context"
	"fmt"
	"os"

	"yapfs/internal/config"
	"yapfs/pkg/utils"

	"github.com/pion/webrtc/v4"
)

// SignalingService orchestrates the complete signaling flow
type SignalingService struct {
	sessionService *SessionService
}

// NewSignalingService creates a new signaling service
func NewSignalingService(cfg *config.Config) *SignalingService {
	// Initialize Firebase client
	client, err := NewClient(context.Background(), &cfg.Firebase)
	if err != nil {
		fmt.Printf("Error: Failed to initialize Firebase client: %v\n", err)
		fmt.Println("Firebase is required for automated SDP exchange. Please check your credentials file and configuration.")
		os.Exit(1)
	}

	sessionService := client.NewSessionService()

	return &SignalingService{
		sessionService: sessionService,
	}
}

// StartSenderSignallingProcess orchestrates the complete sender signaling flow
func (s *SignalingService) StartSenderSignallingProcess(ctx context.Context, peerConn *webrtc.PeerConnection) error {
	// Create offer
	_, err := s.CreateOffer(peerConn)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	// Wait for ICE gathering to complete
	<-webrtc.GatheringCompletePromise(peerConn)

	// Get the final offer with ICE candidates
	finalOffer := peerConn.LocalDescription()
	if finalOffer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Debug: Log the final offer SDP
	fmt.Printf("DEBUG: Final offer SDP:\n%s\n", finalOffer.SDP)
	
	// Encode offer SDP
	encodedOffer, err := utils.EncodeSessionDescription(*finalOffer)
	if err != nil {
		return fmt.Errorf("failed to encode offer SDP: %w", err)
	}

	// Create session with offer
	sessionID, err := s.sessionService.CreateSessionWithOffer(encodedOffer)
	if err != nil {
		return fmt.Errorf("failed to create session with offer: %w", err)
	}

	fmt.Printf("Session created with ID: %s\n", sessionID)

	// Wait for answer from remote peer
	fmt.Printf("Waiting for receiver to answer...")
	answer, err := s.sessionService.CheckForAnswer(ctx, sessionID)
	if err != nil {
		fmt.Printf("Error occurred: %s\n", err)
		return err
	}
	fmt.Printf("Got answer from remote peer: %s\n", answer)

	answerSD, err := utils.DecodeSessionDescription(answer)
	if err != nil {
		return err
	}

	err = peerConn.SetRemoteDescription(answerSD)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	return nil
}

// StartReceiverSignallingProcess orchestrates the complete receiver signaling flow
func (s *SignalingService) StartReceiverSignallingProcess(ctx context.Context, peerConn *webrtc.PeerConnection, sessionID string) error {
	// Get the offer from the session
	encodedOffer, err := s.sessionService.GetOffer(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get offer from session: %w", err)
	}

	// Decode the received offer
	offerSD, err := utils.DecodeSessionDescription(encodedOffer)
	if err != nil {
		return fmt.Errorf("failed to decode offer SDP: %w", err)
	}

	// Debug: Log the received offer SDP
	fmt.Printf("DEBUG: Received offer SDP:\n%s\n", offerSD.SDP)

	// Set remote description
	err = peerConn.SetRemoteDescription(offerSD)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	_, err = s.CreateAnswer(peerConn)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Wait for ICE gathering
	<-webrtc.GatheringCompletePromise(peerConn)

	// Get the final answer with ICE candidates
	finalAnswer := peerConn.LocalDescription()
	if finalAnswer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Encode answer SDP
	encodedAnswer, err := utils.EncodeSessionDescription(*finalAnswer)
	if err != nil {
		return fmt.Errorf("failed to encode answer SDP: %w", err)
	}

	// Upload answer to Firebase
	err = s.sessionService.UpdateAnswer(sessionID, encodedAnswer)
	if err != nil {
		return fmt.Errorf("failed to upload answer to Firebase: %w", err)
	}

	return nil
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