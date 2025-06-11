package app

import (
	"context"
	"fmt"

	"yapfs/internal/config"
	"yapfs/internal/file"
	"yapfs/internal/ui"
	"yapfs/internal/webrtc"
)

// ReceiverOptions configures the receiver application behavior
type ReceiverOptions struct {
	DstPath string // Required: destination path to save received file
	// Future options can be added here:
	// Verbose  bool
	// Timeout  time.Duration
}

// ReceiverApp implements receiver application logic
type ReceiverApp struct {
	config             *config.Config
	peerService        *webrtc.PeerService
	dataChannelService *webrtc.DataChannelService
	signalingService   *webrtc.SignalingService
	ui                 *ui.ConsoleUI
	fileService        *file.FileService
}

// NewReceiverApp creates a new receiver application
func NewReceiverApp(
	cfg *config.Config,
	peerService *webrtc.PeerService,
	dataChannelService *webrtc.DataChannelService,
	signalingService *webrtc.SignalingService,
	ui *ui.ConsoleUI,
	fileService *file.FileService,
) *ReceiverApp {
	return &ReceiverApp{
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui,
		fileService:        fileService,
	}
}

// Run starts the receiver application with the given options
func (r *ReceiverApp) Run(ctx context.Context, opts *ReceiverOptions) error {
	// Validate required options
	if opts.DstPath == "" {
		return fmt.Errorf("destination path is required")
	}

	r.ui.ShowMessage(fmt.Sprintf("Preparing to receive file to: %s", opts.DstPath))

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

	// Setup file receiver with progress and completion tracking channels
	progressCh := make(chan webrtc.ProgressUpdate, 1)
	completionUpdateCh := make(chan webrtc.CompletionUpdate, 1)

	// Handle progress updates
	go func() {
		for update := range progressCh {
			r.ui.UpdateProgress(update.Progress, update.Throughput, update.BytesSent, update.BytesTotal)
		}
	}()

	// Handle completion updates
	go func() {
		for update := range completionUpdateCh {
			if update.Error != nil {
				r.ui.ShowMessage(fmt.Sprintf("Transfer error: %v", update.Error))
			} else {
				r.ui.ShowMessage(update.Message)
			}
		}
	}()

	completionCh, err := r.dataChannelService.SetupFileReceiver(pc, r.fileService, opts.DstPath, progressCh, completionUpdateCh)
	if err != nil {
		return fmt.Errorf("failed to setup file receiver data channel handler: %w", err)
	}

	// Wait for offer SDP from user
	offer, err := r.ui.InputSDP("Offer")
	if err != nil {
		return fmt.Errorf("failed to get offer SDP: %w", err)
	}

	// Decode and set remote description
	offerSD, err := r.signalingService.DecodeSessionDescription(offer)
	if err != nil {
		return fmt.Errorf("failed to decode offer SDP: %w", err)
	}

	err = r.signalingService.SetRemoteDescription(pc, offerSD)
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

	// Encode and display answer SDP for user to copy
	encodedAnswer, err := r.signalingService.EncodeSessionDescription(*finalAnswer)
	if err != nil {
		return fmt.Errorf("failed to encode answer SDP: %w", err)
	}

	err = r.ui.OutputSDP(encodedAnswer, "Answer")
	if err != nil {
		return fmt.Errorf("failed to output answer SDP: %w", err)
	}

	r.ui.ShowMessage("Receiver is ready. Waiting for file transfer...")

	// Wait for either transfer completion or context cancellation
	defer func() {
		close(progressCh)
		close(completionUpdateCh)
	}()

	select {
	case <-completionCh:
		r.ui.ShowMessage("File transfer completed successfully")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
