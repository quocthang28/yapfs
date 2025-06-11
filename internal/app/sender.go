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
	config             *config.Config
	peerService        *webrtc.PeerService
	dataChannelService *webrtc.DataChannelService
	signalingService   *webrtc.SignalingService
	ui                 *ui.ConsoleUI
	fileService        *file.FileService
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
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui,
		fileService:        fileService,
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

	// Create data channel for file transfer with channels
	progressCh := make(chan webrtc.ProgressUpdate, 1)
	completionUpdateCh := make(chan webrtc.CompletionUpdate, 1)

	// Start goroutines to handle channel updates
	go func() {
		for update := range progressCh {
			s.ui.UpdateProgress(update.Progress, update.Throughput, update.BytesSent, update.BytesTotal)
		}
	}()

	go func() {
		for update := range completionUpdateCh {
			if update.Error != nil {
				s.ui.ShowMessage(fmt.Sprintf("Transfer error: %v", update.Error))
			} else {
				s.ui.ShowMessage(update.Message)
			}
		}
	}()

	dataChannel, err := s.dataChannelService.CreateFileSenderDataChannel(pc, "fileTransfer")
	if err != nil {
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	err = s.dataChannelService.SetupFileSender(dataChannel, s.fileService, opts.FilePath, progressCh, completionUpdateCh)
	if err != nil {
		return fmt.Errorf("failed to setup file sender: %w", err)
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

	// Encode and display offer SDP for user to copy
	encodedOffer, err := s.signalingService.EncodeSessionDescription(*finalOffer)
	if err != nil {
		return fmt.Errorf("failed to encode offer SDP: %w", err)
	}

	err = s.ui.OutputSDP(encodedOffer, "Offer")
	if err != nil {
		return fmt.Errorf("failed to output offer SDP: %w", err)
	}

	// Wait for answer SDP from user
	answer, err := s.ui.InputSDP("Answer")
	if err != nil {
		return fmt.Errorf("failed to get answer SDP: %w", err)
	}

	// Decode and set remote description
	answerSD, err := s.signalingService.DecodeSessionDescription(answer)
	if err != nil {
		return fmt.Errorf("failed to decode answer SDP: %w", err)
	}

	err = s.signalingService.SetRemoteDescription(pc, answerSD)
	if err != nil {
		return fmt.Errorf("failed to set remote description: %w", err)
	}

	// Show ready message
	s.ui.ShowMessage("Sender is ready. File will start sending when the data channel opens.")

	// Block until context is cancelled
	defer func() {
		close(progressCh)
		close(completionUpdateCh)
	}()

	<-ctx.Done()
	return ctx.Err()
}
