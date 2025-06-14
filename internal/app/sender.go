package app

import (
	"context"
	"fmt"
	"os"

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
	// Validate required options
	if opts.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	if _, err := os.Stat(opts.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", opts.FilePath)
	}

	s.ui.ShowMessage(fmt.Sprintf("Preparing to send file: %s", opts.FilePath))

	// Create peer connection
	peerConn, err := s.peerService.CreatePeerConnection()
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	defer func() {
		if err := s.peerService.Close(peerConn); err != nil {
			s.ui.ShowMessage(fmt.Sprintf("Error closing peer connection: %v", err))
		}
		if err := s.dataChannelService.Close(); err != nil {
			s.ui.ShowMessage(fmt.Sprintf("Error closing data channel service: %v", err))
		}
	}()

	// Setup connection state handler
	s.peerService.SetupConnectionStateHandler(peerConn, "sender")

	// Start signalling process
	err = s.signalingService.StartSenderSignallingProcess(ctx, peerConn)
	if err != nil {
		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Create data channel for file transfer
	err = s.dataChannelService.CreateFileSenderDataChannel(peerConn, "fileTransfer")
	if err != nil {
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	doneCh, err := s.dataChannelService.SetupFileSender(ctx, opts.FilePath)
	if err != nil {
		return fmt.Errorf("failed to setup file sender: %w", err)
	}

	// Show ready message
	s.ui.ShowMessage("Sender is ready. File will start sending when the data channel opens.")

	// Wait for transfer completion
	<-doneCh
	s.ui.ShowMessage("File transfer completed successfully!")
	return nil
}
