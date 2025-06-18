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

// ReceiverOptions configures the receiver application behavior
type ReceiverOptions struct {
	DestPath string // Required: destination directory or file path to save received file
	// Future options can be added here:
	// Verbose  bool
	// Timeout  time.Duration
}

// ReceiverApp implements receiver application logic
type ReceiverApp struct {
	config             *config.Config
	peerService        *transport.PeerService
	dataChannelService *transport.DataChannelService
	signalingService   *signalling.SignalingService
	ui                 *ui.ConsoleUI
}

// NewReceiverApp creates a new receiver application
func NewReceiverApp(
	cfg *config.Config,
	peerService *transport.PeerService,
	dataChannelService *transport.DataChannelService,
	signalingService *signalling.SignalingService,
	ui *ui.ConsoleUI,
) *ReceiverApp {
	return &ReceiverApp{
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui,
	}
}

// Run starts the receiver application with the given options
func (r *ReceiverApp) Run(ctx context.Context, opts *ReceiverOptions) error {
	// Validate required options
	if opts.DestPath == "" {
		return fmt.Errorf("destination path is required")
	}

	r.ui.ShowMessage(fmt.Sprintf("Preparing to receive file to: %s", opts.DestPath))

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

	peerConn, err := r.peerService.CreatePeerConnection(ctx, "receiver", stateHandler)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}

	// Single cleanup function
	cleanup := func(code string) {
		// Always attempt to clear partial file - DataProcessor will handle the logic
		if err := r.dataChannelService.ClearPartialFile(); err != nil {
			log.Printf("Warning: Failed to clear partial file: %v", err)
		}

		if err := r.dataChannelService.Close(); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing data channel service: %v", err))
		}

		if err := peerConn.Close(); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}

		if code != "" {
			if err := r.signalingService.ClearSession(code); err != nil {
				log.Printf("Warning: Failed to clear Firebase session: %v", err)
			} else {
				log.Printf("Firebase session cleared successfully")
			}
		}
	}

	// Prompt the user to input code (session ID)
	code, err := r.ui.InputCode(ctx)
	if err != nil {
		cleanup("")
		return fmt.Errorf("failed to get code from user: %w", err)
	}

	// Start signalling process
	err = r.signalingService.StartReceiverSignallingProcess(ctx, peerConn.PeerConnection, code)
	if err != nil {
		cleanup(code)
		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Setup file receiver with progress tracking
	doneCh, progressCh, err := r.dataChannelService.SetupFileReceiver(peerConn.PeerConnection, opts.DestPath)
	if err != nil {
		cleanup(code)
		return fmt.Errorf("failed to setup file receiver data channel handler: %w", err)
	}

	// Start updating progress on UI
	consoleUI := ui.NewConsoleUI()
	go consoleUI.StartReceiving(progressCh)

	// Wait for any exit condition
	var exitErr error
	select {
	case <-doneCh:
		// Transfer completed successfully
	case <-ctx.Done():
		// Context cancelled
		exitErr = ctx.Err()
	case exitErr = <-exitCh:
		// Connection closed or error
	}

	cleanup(code)

	return exitErr
}
