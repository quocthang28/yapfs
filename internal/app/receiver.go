// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package app

import (
	"context"
	"fmt"

	"yapfs/internal/config"
	"yapfs/internal/file"
	"yapfs/internal/ui"
	"yapfs/internal/webrtc"
)

// receiverApp implements ReceiverApp interface
type receiverApp struct {
	config           *config.Config
	peerService      webrtc.PeerService
	dataChannelService webrtc.DataChannelService
	signalingService webrtc.SignalingService
	ui               ui.InteractiveUI
	fileService      file.FileService
}

// NewReceiverApp creates a new receiver application
func NewReceiverApp(
	cfg *config.Config,
	peerService webrtc.PeerService,
	dataChannelService webrtc.DataChannelService,
	signalingService webrtc.SignalingService,
	ui ui.InteractiveUI,
	fileService file.FileService,
) ReceiverApp {
	return &receiverApp{
		config:           cfg,
		peerService:      peerService,
		dataChannelService: dataChannelService,
		signalingService: signalingService,
		ui:               ui,
		fileService:      fileService,
	}
}

// Run starts the receiver application
func (r *receiverApp) Run(ctx context.Context) error {
	r.ui.ShowInstructions("receiver")

	// Create peer connection
	pc, err := r.peerService.CreatePeerConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := r.peerService.Close(pc); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}
	}()

	// Setup connection state handler
	r.peerService.SetupConnectionStateHandler(pc, "receiver")

	// Setup data channel handler for receiving
	err = r.dataChannelService.SetupReceiverDataChannelHandler(pc)
	if err != nil {
		return fmt.Errorf("failed to setup receiver data channel handler: %w", err)
	}

	// Wait for offer SDP from user
	offer, err := r.ui.InputSDP("Offer")
	if err != nil {
		return fmt.Errorf("failed to get offer SDP: %w", err)
	}

	// Set remote description
	err = r.signalingService.SetRemoteDescription(pc, offer)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	_, err = r.signalingService.CreateAnswer(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Wait for ICE gathering to complete
	r.ui.ShowMessage("Gathering ICE candidates...")
	err = r.signalingService.WaitForICEGathering(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to gather ICE candidates: %w", err)
	}

	// Get the final answer with ICE candidates
	finalAnswer := pc.LocalDescription()
	if finalAnswer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Display answer SDP for user to copy
	err = r.ui.OutputSDP(*finalAnswer, "Answer")
	if err != nil {
		return fmt.Errorf("failed to output answer SDP: %w", err)
	}

	r.ui.ShowMessage("Receiver is ready. Waiting for data channel connection...")

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// RunWithDest starts the receiver application to save file to destination
func (r *receiverApp) RunWithDest(ctx context.Context, dstPath string) error {
	r.ui.ShowMessage(fmt.Sprintf("Preparing to receive file to: %s", dstPath))

	// Create peer connection
	pc, err := r.peerService.CreatePeerConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := r.peerService.Close(pc); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}
	}()

	// Setup connection state handler
	r.peerService.SetupConnectionStateHandler(pc, "receiver")

	// Setup data channel handler for receiving files with completion signal
	completionCh, err := r.dataChannelService.SetupFileReceiverWithCompletion(pc, r.fileService, dstPath)
	if err != nil {
		return fmt.Errorf("failed to setup file receiver data channel handler: %w", err)
	}

	// Wait for offer SDP from user
	offer, err := r.ui.InputSDP("Offer")
	if err != nil {
		return fmt.Errorf("failed to get offer SDP: %w", err)
	}

	// Set remote description
	err = r.signalingService.SetRemoteDescription(pc, offer)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	_, err = r.signalingService.CreateAnswer(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Wait for ICE gathering to complete
	r.ui.ShowMessage("Gathering ICE candidates...")
	err = r.signalingService.WaitForICEGathering(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to gather ICE candidates: %w", err)
	}

	// Get the final answer with ICE candidates
	finalAnswer := pc.LocalDescription()
	if finalAnswer == nil {
		return fmt.Errorf("local description is nil after ICE gathering")
	}

	// Display answer SDP for user to copy
	err = r.ui.OutputSDP(*finalAnswer, "Answer")
	if err != nil {
		return fmt.Errorf("failed to output answer SDP: %w", err)
	}

	r.ui.ShowMessage("Receiver is ready. Waiting for file transfer...")

	// Wait for either transfer completion or context cancellation
	select {
	case <-completionCh:
		r.ui.ShowMessage("File transfer completed successfully")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}