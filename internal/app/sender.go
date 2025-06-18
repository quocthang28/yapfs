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

	// Single exit channel for all termination conditions
	exitCh := make(chan error, 1)

	// Create peer connection with callback functions
	peerConn, err := s.peerService.CreatePeerConnection(ctx, "sender",
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
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Single cleanup function
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

	// Create data channel for file transfer BEFORE creating the offer
	err = s.dataChannelService.CreateFileSenderDataChannel(peerConn.PeerConnection, "fileTransfer")
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

	// Setup file sender
	progressCh, err := s.dataChannelService.SetupFileSender(ctx, opts.FilePath)
	if err != nil {
		cleanup(sessionID)
		return fmt.Errorf("failed to setup file sender: %w", err)
	}

	// Start updating progress on UI
	go s.ui.StartUpdatingSenderProgress(progressCh)

	// Create transfer completion channel
	transferDone := make(chan error, 1)
	
	// Start file transfer in background
	go func() {
		transferDone <- s.dataChannelService.SendFile()
	}()

	// Wait for any exit condition
	var exitErr error
	select {
	case exitErr = <-transferDone:
		// Transfer completed (successfully or with error)
	case <-ctx.Done():
		exitErr = ctx.Err()
	case exitErr = <-exitCh:
		// Connection closed or error
	}

	cleanup(sessionID)
	return exitErr
}
