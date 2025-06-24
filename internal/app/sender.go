package app

import (
	"context"
	"fmt"
	"log"

	"yapfs/internal/config"
	"yapfs/internal/reporter"
	"yapfs/internal/signalling"
	"yapfs/internal/transport"
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
	config           *config.Config
	peerService      *transport.PeerService
	signalingService *signalling.SignalingService
}

// NewSenderApp creates a new sender application
func NewSenderApp(cfg *config.Config, peerService *transport.PeerService, signalingService *signalling.SignalingService) *SenderApp {
	return &SenderApp{
		config:           cfg,
		peerService:      peerService,
		signalingService: signalingService,
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

	// Create data channel for file transfer with completion callback
	senderChannel, err := transport.CreateSenderChannel(ctx, s.config, peerConn.PeerConnection, "fileTransfer", opts.FilePath, 
		func(err error) {
			// Channel completed or failed - signal app to exit
			select {
			case exitCh <- err:
			default:
			}
		})
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
		progressCh := senderChannel.StartMessageLoop()

		propressReporter := reporter.NewProgressReporter()
		propressReporter.StartUpdatingProgress(ctx, progressCh)
	}()

	// Wait for any exit condition
	var exitErr error

	select {
	case exitErr = <-exitCh:
		// Transfer completed, connection closed, or error
		if exitErr == nil {
			log.Println("File transfer completed successfully")
		}
	case <-ctx.Done():
		exitErr = ctx.Err()
	}

	cleanup(sessionID)

	return exitErr
}
