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

	// Single exit channel for all termination conditions
	exitCh := make(chan error, 1)

	// Create peer connection with state handler
	stateHandler := transport.CreateDefaultStateHandler(
		func(err error) {
			log.Printf("Peer connection error: %v", err)
			select {
			case exitCh <- err:
			default:
			}
		},
		func() {},
		func() {
			// Connection closed - signal app exit
			select {
			case exitCh <- nil:
			default:
			}
		},
	)

	peerConn, err := s.peerService.CreatePeerConnection(ctx, "sender", stateHandler)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Single cleanup function
	cleanup := func(sessionID string) {
		if err := s.dataChannelService.Close(); err != nil {
			log.Printf("Error closing data channel service: %v", err)
		}

		if err := peerConn.Close(); err != nil {
			log.Printf("Error closing peer connection: %v", err)
		}

		if sessionID != "" {
			if err := s.signalingService.ClearSession(sessionID); err != nil {
				log.Printf("Warning: Failed to clear Firebase session: %v", err)
			} else {
				log.Printf("Firebase session cleared successfully")
			}
		}
	}

	// Create data channel for file transfer BEFORE creating the offer
	err = s.dataChannelService.CreateFileSenderDataChannel(peerConn.PeerConnection, "fileTransfer")
	if err != nil {
		cleanup("")
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	// Start signalling process
	sessionID, err := s.signalingService.StartSenderSignallingProcess(ctx, peerConn.PeerConnection)
	if err != nil {
		cleanup("")
		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Setup file sender with progress
	doneCh, progressCh, err := s.dataChannelService.SetupFileSender(ctx, opts.FilePath)
	if err != nil {
		cleanup(sessionID)
		return fmt.Errorf("failed to setup file sender: %w", err)
	}

	// Start updating progress on UI
	go s.updateProgress(opts, progressCh)

	// Wait for any exit condition
	var exitErr error
	select {
	case <-doneCh:
		// Transfer completed successfully
	case <-ctx.Done():
		exitErr = ctx.Err()
	case exitErr = <-exitCh:
		// Connection closed or error
	}

	cleanup(sessionID)
	return exitErr
}

func (s *SenderApp) updateProgress(opts *SenderOptions, progressCh <-chan transport.ProgressUpdate) {
	progressUI := ui.NewProgressUI()
	filename := filepath.Base(opts.FilePath)

	var started bool
	for update := range progressCh {
		if !started {
			progressUI.StartProgressSending(filename, update.BytesTotal)
			started = true
		}
		progressUI.UpdateProgress(update)

		// Complete progress when transfer finishes
		if update.Percentage >= 100.0 {
			progressUI.CompleteProgress()
		}
	}
}
