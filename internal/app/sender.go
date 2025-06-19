package app

import (
	"context"
	"fmt"
	"log"

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
) *SenderApp {
	return &SenderApp{
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui.NewConsoleUI("Sending"),
	}
}

// Run starts the sender application with the given options
func (s *SenderApp) Run(ctx context.Context, opts *SenderOptions) error {
	log.Printf("Preparing to send file: %s", opts.FilePath)

	// Single channel for all exit conditions
	exitCh := make(chan error, 1)

	// Create peer connection with callback functions
	peerConn, err := s.peerService.CreatePeerConnection(ctx, "sender",
		func(err error) {
			// onError
			log.Printf("Peer connection error: %v", err)
			select {
			case exitCh <- err:
			default:
			}
		},
		func() {
			// onConnected
		},
		func() {
			// onClosed
			select {
			case exitCh <- nil:
			default:
			}
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Cleanup function
	cleanup := func(sessionID string) {
		if err := peerConn.Close(); err != nil {
			log.Printf("Error closing peer connection: %v", err)
		}

		if sessionID != "" {
			if err := s.signalingService.ClearSession(ctx, sessionID); err != nil {
				log.Printf("Warning: Failed to clear Firebase session: %v", err)
			}
		}
	}

	// Create data channel for file transfer and initialize everything
	err = s.dataChannelService.CreateFileSenderDataChannel(ctx, peerConn.PeerConnection, "fileTransfer", opts.FilePath)
	if err != nil {
		cleanup("")
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	// Start signalling process
	sessionID, err := s.signalingService.StartSenderSignallingProcess(ctx, peerConn.PeerConnection)
	if err != nil {
		cleanup(sessionID)

		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Start file transfer in background
	go func() {
		progressCh, err := s.dataChannelService.SendFile()
		if err != nil {
			exitCh <- err
			return
		}

		// Start updating progress on UI,
		// this blocks until transfer is complete or context is cancelled
		s.ui.StartUpdatingSenderProgress(ctx, progressCh)
		exitCh <- nil
	}()

	// Wait for any exit condition
	var exitErr error
	select {
	case exitErr = <-exitCh:
		// Transfer completed, connection closed, or error
	case <-ctx.Done():
		exitErr = ctx.Err()
	}

	cleanup(sessionID)

	return exitErr
}
