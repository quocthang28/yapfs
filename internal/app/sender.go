package app

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"yapfs/internal/config"
	"yapfs/internal/signalling"
	"yapfs/internal/transport"
	"yapfs/internal/ui"
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
	peerService        *transport.PeerService
	dataChannelService *transport.DataChannelService
	signalingService   *signalling.SignalingService
	ui                 *ui.ConsoleUI
}

// NewSenderApp creates a new sender application
func NewSenderApp(
	cfg *config.Config,
	peerService *transport.PeerService,
	dataChannelService *transport.DataChannelService,
	signalingService *signalling.SignalingService,
	ui *ui.ConsoleUI,
) *SenderApp {
	return &SenderApp{
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui,
	}
}

// Run starts the sender application with the given options
func (s *SenderApp) Run(ctx context.Context, opts *SenderOptions) error {
	log.Printf("Preparing to send file: %s", opts.FilePath)

	// Create peer connection
	peerConn, err := s.peerService.CreatePeerConnection()
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := s.peerService.Close(peerConn); err != nil {
			log.Printf("Error closing peer connection: %v", err)
		}
		if err := s.dataChannelService.Close(); err != nil {
			log.Printf("Error closing data channel service: %v", err)
		}
	}()

	// Setup connection state handler
	s.peerService.SetupConnectionStateHandler(peerConn, "sender")

	// Create data channel for file transfer BEFORE creating the offer
	err = s.dataChannelService.CreateFileSenderDataChannel(peerConn, "fileTransfer")
	if err != nil {
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	// Start signalling process
	sessionID, err := s.signalingService.StartSenderSignallingProcess(ctx, peerConn)
	if err != nil {
		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Ensure Firebase session is cleared regardless of how the function exits
	defer func() {
		if sessionID != "" {
			if err := s.signalingService.ClearSession(sessionID); err != nil {
				log.Printf("Warning: Failed to clear Firebase session: %v", err)
			} else {
				log.Printf("Firebase session cleared successfully")
			}
		}
	}()

	// Setup file sender with progress
	doneCh, progressCh, err := s.dataChannelService.SetupFileSender(ctx, opts.FilePath)
	if err != nil {
		return fmt.Errorf("failed to setup file sender: %w", err)
	}

	// Create progress UI
	progressUI := ui.NewProgressUI()
	filename := filepath.Base(opts.FilePath)

	// Show ready message
	s.ui.ShowMessage("Sender is ready. File will start sending when the data channel opens.")

	// Start progress tracking
	go func() {
		var started bool
		for update := range progressCh {
			if !started {
				progressUI.StartProgress(filename, update.BytesTotal)
				started = true
			}
			progressUI.UpdateProgress(update)

			// Show final summary when transfer completes
			if update.Percentage >= 100.0 {
				progressUI.CompleteProgress()
				progressUI.ShowTransferSummary(update)
			}
		}
	}()

	// Wait for transfer completion
	select {
	case <-doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
