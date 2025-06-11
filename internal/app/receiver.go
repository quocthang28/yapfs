package app

import (
	"fmt"

	"yapfs/internal/config"
	"yapfs/internal/processor"
	"yapfs/internal/transport"
	"yapfs/internal/ui"
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
	peerService        *transport.PeerService
	dataChannelService *transport.DataChannelService
	signalingService   *transport.SignalingService
	ui                 *ui.ConsoleUI
	dataProcessor      *processor.DataProcessor
}

// NewReceiverApp creates a new receiver application
func NewReceiverApp(
	cfg *config.Config,
	peerService *transport.PeerService,
	dataChannelService *transport.DataChannelService,
	signalingService *transport.SignalingService,
	ui *ui.ConsoleUI,
	dataProcessor *processor.DataProcessor,
) *ReceiverApp {
	return &ReceiverApp{
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui,
		dataProcessor:      dataProcessor,
	}
}

// Run starts the receiver application with the given options
func (r *ReceiverApp) Run(opts *ReceiverOptions) error {
	// Validate required options
	if opts.DstPath == "" {
		return fmt.Errorf("destination path is required")
	}

	r.ui.ShowMessage(fmt.Sprintf("Preparing to receive file to: %s", opts.DstPath))

	// Create peer connection
	peerConn, err := r.peerService.CreatePeerConnection()
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := r.peerService.Close(peerConn); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}
	}()

	// Setup connection state handler
	r.peerService.SetupConnectionStateHandler(peerConn, "receiver")

	// Setup file receiver
	doneCh, err := r.dataChannelService.SetupFileReceiver(peerConn, r.dataProcessor, opts.DstPath)
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

	err = r.signalingService.SetRemoteDescription(peerConn, offerSD)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Create answer
	_, err = r.signalingService.CreateAnswer(peerConn)
	if err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Wait for ICE gathering to complete
	r.ui.ShowMessage("Gathering ICE candidates...")
	<-r.signalingService.WaitForICEGathering(peerConn)

	// Get the final answer with ICE candidates
	finalAnswer := peerConn.LocalDescription()
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

	// Wait for transfer completion
	<-doneCh
	r.ui.ShowMessage("File transfer completed successfully")
	return nil
}
