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
func (s *SignalingService) StartSenderSignallingProcess(peerConn *webrtc.PeerConnection) error {
	// Create session
	// TODO: create both session and offer before uploading to firebase
	sessionID, err := s.sessionService.CreateSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// TODO: return this to UI
	fmt.Printf("Session created with ID: %s\n", sessionID)

	// Create offer
	_, err = s.CreateOffer(peerConn)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	// Wait for ICE gathering
	<-webrtc.GatheringCompletePromise(peerConn)

	// Get the final offer with ICE candidates
	finalOffer := peerConn.LocalDescription()
	if finalOffer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Encode offer SDP
	encodedOffer, err := utils.EncodeSessionDescription(*finalOffer)
	if err != nil {
		return fmt.Errorf("failed to encode offer SDP: %w", err)
	}

	// Upload offer to Firebase
	s.sessionService.UpdateOffer(encodedOffer)

	// Wait for answer from remote peer
	fmt.Printf("Waiting for receiver to answer...")
	answerChan, errorChan := s.sessionService.CheckForAnswer()
	var answer string

answerLoop:
	for {
		select {
		case answer = <-answerChan:
			fmt.Printf("Got answer from remote peer: %s\n", answer)
			break answerLoop
		case err := <-errorChan:
			fmt.Printf("Error occurred: %s\n", err)
			return err
		}
	}

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
func (s *SignalingService) StartReceiverSignallingProcess(peerConn *webrtc.PeerConnection) error {
	// Ask for session code

	// Get session

	// Decode the received offer
	offerSD, err := utils.DecodeSessionDescription(encodedOffer)
	if err != nil {
		return fmt.Errorf("failed to decode offer SDP: %w", err)
	}

	// Set remote description
	err = s.setRemoteDescription(peerConn, offerSD)
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

	// Upload answer to firebase
	
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

// SetRemoteDescription sets the remote session description
func (s *SignalingService) setRemoteDescription(peerConn *webrtc.PeerConnection, sd webrtc.SessionDescription) error {
	err := peerConn.SetRemoteDescription(sd)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}
	return nil
}
