package app

import (
	"context"
	"fmt"
	"log"

	"yapfs/internal/config"
	"yapfs/internal/reporter"
	"yapfs/internal/signalling"
	"yapfs/internal/transport"
	"yapfs/pkg/utils"
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
	config           *config.Config
	peerService      *transport.PeerService
	signalingService *signalling.SignalingService
}

// NewReceiverApp creates a new receiver application
func NewReceiverApp(cfg *config.Config, peerService *transport.PeerService, signalingService *signalling.SignalingService) *ReceiverApp {
	return &ReceiverApp{
		config:           cfg,
		peerService:      peerService,
		signalingService: signalingService,
	}
}

// Run starts the receiver application with the given options
func (r *ReceiverApp) Run(ctx context.Context, opts *ReceiverOptions) error {
	// Validate required options
	if opts.DestPath == "" {
		return fmt.Errorf("destination path is required")
	}

	log.Printf("Preparing to receive file to: %s", opts.DestPath)

	// Single exit channel for all termination conditions
	exitCh := make(chan error, 1)

	// Create peer connection with callback functions
	peerConn, err := r.peerService.CreatePeerConnection(ctx, "receiver",
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
	cleanup := func(code string) {
		if err := peerConn.Close(); err != nil {
			log.Printf("Error closing peer connection: %v", err)
		}

		if code != "" {
			if err := r.signalingService.ClearSession(ctx, code); err != nil {
				log.Printf("Warning: Failed to clear Firebase session: %v", err)
			}
		}
	}

	// Prompt the user to input code (session ID)
	code, err := utils.AskForCode(ctx)
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

	// Create receiver channel for file transfer with completion callback
	receiverChannel, err := transport.CreateReceiverChannel(ctx, r.config, peerConn.PeerConnection, opts.DestPath,
		func(err error) {
			// Channel completed or failed - signal app to exit
			select {
			case exitCh <- err:
			default:
			}
		})
	if err != nil {
		cleanup(code)
		return fmt.Errorf("failed to create file receiver data channel: %w", err)
	}

	// Start file receive with progress tracking in background
	go func() {
		progressCh := receiverChannel.StartMessageLoop()

		propressReporter := reporter.NewProgressReporter()
		propressReporter.StartUpdatingProgress(ctx, progressCh)
	}()

	// Wait for any exit condition
	var exitErr error

	select {
	case exitErr = <-exitCh:
		// File receive completed or error
		if exitErr == nil {
			log.Println("File transfer completed successfully")
		}
	case <-ctx.Done():
		// Context cancelled
		exitErr = ctx.Err()
	}

	cleanup(code)

	return exitErr
}
