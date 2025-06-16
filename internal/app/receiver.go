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

	// Data processor is now handled internally by the data channel service

	// Create peer connection
	peerConn, err := r.peerService.CreatePeerConnection()
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := r.peerService.Close(peerConn); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}
		if err := r.dataChannelService.Close(); err != nil {
			r.ui.ShowMessage(fmt.Sprintf("Error closing data channel service: %v", err))
		}
	}()

	// Setup connection state handler
	r.peerService.SetupConnectionStateHandler(peerConn, "receiver")

	// Prompt the user to input code (session ID)
	code, err := r.ui.InputCode()
	if err != nil {
		return fmt.Errorf("failed to get code from user: %w", err)
	}

	// Start signalling process
	err = r.signalingService.StartReceiverSignallingProcess(ctx, peerConn, code)
	if err != nil {
		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Setup file receiver with progress tracking
	doneCh, progressCh, err := r.dataChannelService.SetupFileReceiver(peerConn, opts.DestPath)
	if err != nil {
		return fmt.Errorf("failed to setup file receiver data channel handler: %w", err)
	}

	r.ui.ShowMessage("Receiver is ready. Waiting for file transfer...")

	// Create progress UI  
	progressUI := ui.NewProgressUI()

	// Start progress tracking
	go func() {
		var started bool
		for update := range progressCh {
			if !started {
				// Get filename from metadata when available
				var filename string
				if update.MetaData.Name != "" {
					filename = update.MetaData.Name
				} else {
					filename = filepath.Base(opts.DestPath)
				}
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
	<-doneCh
	r.ui.ShowMessage("File transfer completed successfully!")
	
	// Clear Firebase session after successful transfer
	if code != "" {
		if err := r.signalingService.ClearSession(code); err != nil {
			log.Printf("Warning: Failed to clear Firebase session: %v", err)
		} else {
			log.Printf("Firebase session cleared successfully")
		}
	}
	
	return nil
}
