// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"yapfs/internal/config"
	"yapfs/internal/file"
	"yapfs/internal/ui"
	"yapfs/internal/webrtc"

	"github.com/spf13/cobra"
)

var (
	cfg *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "yapfs",
	Short: "YAPFS - Yet Another P2P File Sharing utility",
	Long: `YAPFS is a peer-to-peer file sharing utility using WebRTC data channels.

This application allows secure, direct file transfers between two machines without 
requiring a central server. Files are transferred directly using WebRTC with proper 
flow control and progress monitoring.

Usage:
  Send a file:    yapfs send --file /path/to/file
  Receive a file: yapfs receive --dst /path/to/save/file

Both peers will exchange SDP offers/answers manually to establish the connection.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		cfg = config.NewDefaultConfig()
		if err := cfg.Validate(); err != nil {
			log.Fatalf("Invalid configuration: %v", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// createContext creates a context that cancels on interrupt signals
func createContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	return ctx
}

// createServices creates and wires up all the application services
func createServices() (webrtc.PeerService, webrtc.DataChannelService, webrtc.SignalingService, ui.InteractiveUI, file.FileService) {
	// Create connection state handler
	stateHandler := &webrtc.DefaultConnectionStateHandler{}

	// Create throughput reporter
	throughputReporter := &webrtc.DefaultThroughputReporter{}

	// Create services
	signalingService := webrtc.NewSignalingService()
	peerService := webrtc.NewPeerService(cfg, stateHandler)
	dataChannelService := webrtc.NewDataChannelService(cfg, throughputReporter)
	ui := ui.NewConsoleUI(signalingService)
	fileService := file.NewFileService()

	return peerService, dataChannelService, signalingService, ui, fileService
}
