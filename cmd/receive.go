package cmd

import (
	"context"
	"fmt"
	"log"
	"yapfs/internal/app"
	"yapfs/pkg/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ReceiveFlags struct {
	DestPath string
	// Future flags can be easily added here:
	// Verbose  bool
	// Timeout  int
	// Port     int
}

var receiveFlags ReceiveFlags

// receiveCmd represents the receive command
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive a file from peer (responds to offer)",
	Long: `Receive a file from a peer via WebRTC. This will:

1. Create a WebRTC peer connection
2. Wait for an SDP offer from the sender
3. Generate an SDP answer
4. Receive file metadata (name, size, type)
5. Save the file with its original name once connected

Use --dst to specify directory where to save the received file (defaults to current directory).`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateReceiveFlags(&receiveFlags)
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Starting receiver, will save to: %s", receiveFlags.DestPath)
		if err := runReceiverApp(&receiveFlags); err != nil {
			log.Fatalf("Receiver failed: %v", err)
		}
	},
}

// validateReceiveFlags validates the receive command flags
func validateReceiveFlags(flags *ReceiveFlags) error {
	if flags.DestPath == "" {
		flags.DestPath = "." // Default to current directory
	}

	// Resolve and validate destination path
	resolvedPath, err := utils.ResolveDestinationPath(flags.DestPath)
	if err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	// Update the flag with the resolved path
	flags.DestPath = resolvedPath

	// Future validations can be easily added here:
	// if flags.Timeout <= 0 {
	//     return fmt.Errorf("timeout must be positive")
	// }
	return nil
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	// Define flags with struct binding
	receiveCmd.Flags().StringVarP(&receiveFlags.DestPath, "dst", "d", ".", "Destination directory to save received file (defaults to current directory)")

	// Bind flags to viper for environment variable support
	viper.BindPFlag("receive.dst", receiveCmd.Flags().Lookup("dst"))

	// Future flag bindings can be easily added here:
	// viper.BindPFlag("receive.verbose", receiveCmd.Flags().Lookup("verbose"))
	// viper.BindPFlag("receive.timeout", receiveCmd.Flags().Lookup("timeout"))
}

// runReceiverApp creates and runs the receiver application
func runReceiverApp(flags *ReceiveFlags) error {
	peerService, dataChannelService, signalingService, ui := createServices()

	// Future flag processing can be easily added here:
	// if flags.Verbose {
	//     log.Printf("Verbose mode enabled")
	//     // Configure services for verbose logging
	// }
	// if flags.Timeout > 0 {
	//     // Apply timeout to context or services
	// }

	// Create receiver options from flags
	opts := &app.ReceiverOptions{
		DestPath: flags.DestPath,
	}

	receiverApp := app.NewReceiverApp(cfg, peerService, dataChannelService, signalingService, ui)
	ctx := context.Background()
	return receiverApp.Run(ctx, opts)
}
