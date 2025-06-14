package cmd

import (
	"log"
	"os"

	"yapfs/internal/config"
	"yapfs/internal/signalling"
	"yapfs/internal/transport"
	"yapfs/internal/ui"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
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
		// Load .env file if it exists
		gotenv.Load(".env")

		// Initialize viper configuration
		initConfig()

		// Initialize configuration with defaults
		cfg = config.NewDefaultConfig()

		// Only unmarshal if a config file was actually found and read
		if viper.ConfigFileUsed() != "" {
			// Unmarshal config from file, overriding defaults
			if err := viper.Unmarshal(cfg); err != nil {
				log.Fatalf("Failed to unmarshal config: %v", err)
			}
			
			// Manually override Firebase config as workaround for Viper unmarshal issue
			if projectID := viper.GetString("firebase.project_id"); projectID != "" {
				cfg.Firebase.ProjectID = projectID
			}
			if databaseURL := viper.GetString("firebase.database_url"); databaseURL != "" {
				cfg.Firebase.DatabaseURL = databaseURL
			}
			if credentialsPath := viper.GetString("firebase.credentials_path"); credentialsPath != "" {
				cfg.Firebase.CredentialsPath = credentialsPath
			}
		}

		// Validate the final configuration
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
		// Set up Viper to look for config.json specifically
		viper.SetConfigName("config") // name of config file (without extension)
		viper.SetConfigType("json")   // REQUIRED if the config file does not have the extension in the name
		
		// Add search paths
		viper.AddConfigPath(".")      // look in current directory first
		viper.AddConfigPath("./config") // look in config subdirectory
		
		// Search in home directory
		if home, err := os.UserHomeDir(); err == nil {
			viper.AddConfigPath(home) // look in home directory
		}
	}

	// Read in environment variables that match
	viper.AutomaticEnv()

	// Find and read the config file
	viper.ReadInConfig()
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// createContext creates a context that cancels on interrupt signals
// func createContext() context.Context {
// 	ctx, cancel := context.WithCancel(context.Background())

// 	// Setup signal handling
// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// 	go func() {
// 		<-sigChan
// 		fmt.Println("\nReceived interrupt signal, shutting down...")
// 		cancel()
// 	}()

// 	return ctx
// }

// createServices creates and wires up all the application services
func createServices() (*transport.PeerService, *transport.DataChannelService, *signalling.SignalingService, *ui.ConsoleUI) {
	// Create connection state handler
	stateHandler := &transport.DefaultConnectionStateHandler{}

	// Create services
	signalingService := signalling.NewSignalingService(cfg)
	peerService := transport.NewPeerService(cfg, stateHandler)
	consoleUI := ui.NewConsoleUI()
	dataChannelService := transport.NewDataChannelService(cfg)

	return peerService, dataChannelService, signalingService, consoleUI
}
