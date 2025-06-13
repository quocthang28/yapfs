package app

import (
	"fmt"
	"os"

	"yapfs/internal/config"
	"yapfs/internal/processor"
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
	dataProcessor      *processor.DataProcessor
}

// NewSenderApp creates a new sender application
func NewSenderApp(
	cfg *config.Config,
	peerService *transport.PeerService,
	dataChannelService *transport.DataChannelService,
	signalingService *signalling.SignalingService,
	ui *ui.ConsoleUI,
	dataProcessor *processor.DataProcessor,
) *SenderApp {
	return &SenderApp{
		config:             cfg,
		peerService:        peerService,
		dataChannelService: dataChannelService,
		signalingService:   signalingService,
		ui:                 ui,
		dataProcessor:      dataProcessor,
	}
}

// Run starts the sender application with the given options
func (s *SenderApp) Run(opts *SenderOptions) error {
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
	}()

	// Setup connection state handler
	s.peerService.SetupConnectionStateHandler(peerConn, "sender")

	// Start signalling process
	err = s.signalingService.StartSenderSignallingProcess(peerConn)
	if err != nil {
		return fmt.Errorf("failed during signalling process: %w", err)
	}

	// Prepare file for sending using DataProcessor //TODO: data processor to be handled internally by data channel service
	err = s.dataProcessor.PrepareFileForSending(opts.FilePath)
	if err != nil {
		return fmt.Errorf("failed to prepare file for sending: %w", err)
	}
	defer s.dataProcessor.Close()

	// Create data channel for file transfer
	err = s.dataChannelService.CreateFileSenderDataChannel(peerConn, "fileTransfer")
	if err != nil {
		return fmt.Errorf("failed to create file sender data channel: %w", err)
	}

	doneCh, err := s.dataChannelService.SetupFileSender(s.dataProcessor)
	if err != nil {
		return fmt.Errorf("failed to setup file sender: %w", err)
	}

	// // Create offer
	// _, err = s.signalingService.CreateOffer(peerConn)
	// if err != nil {
	// 	return fmt.Errorf("failed to create offer: %w", err)
	// }

	// // Wait for ICE gathering to complete // TODO: trickle ICE
	// s.ui.ShowMessage("Gathering ICE candidates...")
	// <-s.signalingService.WaitForICEGathering(peerConn)

	// // Get the final offer with ICE candidates
	// finalOffer := peerConn.LocalDescription()
	// if finalOffer == nil {
	// 	return fmt.Errorf("local description is nil after ICE gathering")
	// }

	// // Encode and display offer SDP for user to copy
	// encodedOffer, err := utils.EncodeSessionDescription(*finalOffer)
	// if err != nil {
	// 	return fmt.Errorf("failed to encode offer SDP: %w", err)
	// }

	// err = s.ui.OutputSDP(encodedOffer, "Offer")
	// if err != nil {
	// 	return fmt.Errorf("failed to output offer SDP: %w", err)
	// }

	// // Wait for answer SDP from user
	// answer, err := s.ui.InputSDP("Answer")
	// if err != nil {
	// 	return fmt.Errorf("failed to get answer SDP: %w", err)
	// }

	// // Decode and set remote description
	// answerSD, err := utils.DecodeSessionDescription(answer)
	// if err != nil {
	// 	return fmt.Errorf("failed to decode answer SDP: %w", err)
	// }

	// err = s.signalingService.SetRemoteDescription(peerConn, answerSD)
	// if err != nil {
	// 	return fmt.Errorf("failed to set remote description: %w", err)
	// }

	// Show ready message
	s.ui.ShowMessage("Sender is ready. File will start sending when the data channel opens.")

	// Wait for transfer completion
	<-doneCh
	s.ui.ShowMessage("File transfer completed successfully!")
	return nil
}
