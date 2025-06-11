package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"yapfs/internal/config"
	"yapfs/internal/processor"
	"yapfs/internal/transport"
	"yapfs/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfg     *config.Config
	cfgFile string
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
		// Initialize viper configuration
		initConfig()

		// Initialize configuration
		cfg = config.NewDefaultConfig()
		if err := cfg.Validate(); err != nil {
			log.Fatalf("Invalid configuration: %v", err)
		}
	},
}

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.yapfs.yaml)")

	// Set up viper environment variable support
	viper.SetEnvPrefix("YAPFS")
	viper.AutomaticEnv()
}

// initConfig reads in config file and ENV variables
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Warning: Could not find home directory: %v", err)
			return
		}

		// Search config in home directory with name ".yapfs" (without extension)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".yapfs")
	}

	// Read in environment variables that match
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		log.Printf("Using config file: %s", viper.ConfigFileUsed())
	}
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
func createServices() (*transport.PeerService, *transport.DataChannelService, *transport.SignalingService, *ui.ConsoleUI, *processor.DataProcessor) {
	// Create connection state handler
	stateHandler := &transport.DefaultConnectionStateHandler{}

	// Create services
	signalingService := transport.NewSignalingService()
	peerService := transport.NewPeerService(cfg, stateHandler)
	consoleUI := ui.NewConsoleUI()

	dataChannelService := transport.NewDataChannelService(cfg)
	dataProcessor := processor.NewDataProcessor()

	return peerService, dataChannelService, signalingService, consoleUI, dataProcessor
}
