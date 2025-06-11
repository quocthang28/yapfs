// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"fmt"
	"os"

	"yapfs/internal/config"
	"yapfs/internal/file"
	"yapfs/internal/ui"
	"yapfs/internal/webrtc"
)

// SenderOptions configures the sender application behavior
type SenderOptions struct {
	FilePath string // Required: path to file to send
	// Future options can be added here:
	// Verbose  bool
	// Timeout  time.Duration
}

// SenderApp implements sender application logic
type SenderApp struct {
	config           *config.Config
	peerService      *webrtc.PeerService
	dataChannelService *webrtc.DataChannelService
	signalingService *webrtc.SignalingService
	ui               *ui.ConsoleUI
	fileService      *file.FileService
}

// NewSenderApp creates a new sender application
func NewSenderApp(
	cfg *config.Config,
	peerService *webrtc.PeerService,
	dataChannelService *webrtc.DataChannelService,
	signalingService *webrtc.SignalingService,
	ui *ui.ConsoleUI,
	fileService *file.FileService,
) *SenderApp {
	return &SenderApp{
		config:           cfg,
		peerService:      peerService,
		dataChannelService: dataChannelService,
		signalingService: signalingService,
		ui:               ui,
		fileService:      fileService,
	}
}

// Run starts the sender application with the given options
func (s *SenderApp) Run(ctx context.Context, opts *SenderOptions) error {
	// Validate required options
	if opts.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	if _, err := os.Stat(opts.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", opts.FilePath)
	}

	s.ui.ShowMessage(fmt.Sprintf("Preparing to send file: %s", opts.FilePath))

	// Create peer connection
	pc, err := s.peerService.CreatePeerConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := s.peerService.Close(pc); err != nil {
			s.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}
	}()

	// Setup connection state handler
	s.peerService.SetupConnectionStateHandler(pc, "sender")

	// Create data channel for file transfer
	_, err = s.dataChannelService.CreateFileSenderDataChannel(pc, "fileTransfer", s.fileService, opts.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	// Create offer
	_, err = s.signalingService.CreateOffer(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	// Wait for ICE gathering to complete
	s.ui.ShowMessage("Gathering ICE candidates...")
	err = s.signalingService.WaitForICEGathering(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to gather ICE candidates: %w", err)
	}

	// Get the final offer with ICE candidates
	finalOffer := pc.LocalDescription()
	if finalOffer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Display offer SDP for user to copy
	err = s.ui.OutputSDP(*finalOffer, "Offer")
	if err != nil {
		return fmt.Errorf("failed to output offer SDP: %w", err)
	}

	// Wait for answer SDP from user
	answer, err := s.ui.InputSDP("Answer")
	if err != nil {
		return fmt.Errorf("failed to get answer SDP: %w", err)
	}

	// Set remote description
	err = s.signalingService.SetRemoteDescription(pc, answer)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Show ready message
	s.ui.ShowMessage("Sender is ready. File will start sending when the data channel opens.")

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

