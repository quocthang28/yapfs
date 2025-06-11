package cmd

import (
	"fmt"
	"log"
	"yapfs/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type SendFlags struct {
	FilePath string
	// Future flags can be easily added here:
	// Verbose  bool
	// Timeout  int
	// Port     int
}

var sendFlags SendFlags

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a file to peer (creates offer)",
	Long: `Send a file to a peer via WebRTC. This will:

1. Create a WebRTC peer connection
2. Generate an SDP offer 
3. Wait for you to exchange SDP with the receiver
4. Send the specified file once connected

Use --file to specify the path to the file you want to send.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateSendFlags(&sendFlags)
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Starting sender for file: %s", sendFlags.FilePath)
		if err := runSenderApp(&sendFlags); err != nil {
			log.Fatalf("Sender failed: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	
	// Define flags with struct binding
	sendCmd.Flags().StringVarP(&sendFlags.FilePath, "file", "f", "", "Path to file to send (required)")
	
	// Mark required flags
	sendCmd.MarkFlagRequired("file")
	
	// Bind flags to viper for environment variable support
	viper.BindPFlag("send.file", sendCmd.Flags().Lookup("file"))
	
	// Future flag bindings can be easily added here:
	// viper.BindPFlag("send.verbose", sendCmd.Flags().Lookup("verbose"))
	// viper.BindPFlag("send.timeout", sendCmd.Flags().Lookup("timeout"))
}

// validateSendFlags validates the send command flags
func validateSendFlags(flags *SendFlags) error {
	if flags.FilePath == "" {
		return fmt.Errorf("file path is required")
	}
	// Future validations can be easily added here:
	// if flags.Timeout <= 0 {
	//     return fmt.Errorf("timeout must be positive")
	// }
	return nil
}

// runSenderApp creates and runs the sender application
func runSenderApp(flags *SendFlags) error {
	ctx := createContext()
	peerService, dataChannelService, signalingService, ui, fileService := createServices()

	// Future flag processing can be easily added here:
	// if flags.Verbose {
	//     log.Printf("Verbose mode enabled")
	//     // Configure services for verbose logging
	// }
	// if flags.Timeout > 0 {
	//     // Apply timeout to context or services
	// }

	// Create sender options from flags
	opts := &app.SenderOptions{
		FilePath: flags.FilePath,
	}

	senderApp := app.NewSenderApp(cfg, peerService, dataChannelService, signalingService, ui, fileService)
	return senderApp.Run(ctx, opts)
}
