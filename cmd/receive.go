package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"yapfs/internal/app"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type ReceiveFlags struct {
	DstPath string
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
4. Receive and save the file once connected

Use --dst to specify where to save the received file.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return validateReceiveFlags(&receiveFlags)
	},
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("Starting receiver, will save to: %s", receiveFlags.DstPath)
		if err := runReceiverApp(&receiveFlags); err != nil {
			log.Fatalf("Receiver failed: %v", err)
		}
	},
}

// validateReceiveFlags validates the receive command flags
func validateReceiveFlags(flags *ReceiveFlags) error {
	if flags.DstPath == "" {
		return fmt.Errorf("destination path is required")
	}
	// Future validations can be easily added here:
	// if flags.Timeout <= 0 {
	//     return fmt.Errorf("timeout must be positive")
	// }
	return validateDstPath(flags.DstPath)
}

// validateDstPath ensures the destination path is valid for file creation
func validateDstPath(dstPath string) error {
	// Check if path exists and is a directory
	if info, err := os.Stat(dstPath); err == nil {
		if info.IsDir() {
			return fmt.Errorf("destination path '%s' is a directory, please specify a file path", dstPath)
		}
		// If file exists, it will be overwritten - this is acceptable
		return nil
	}

	// If path doesn't exist, check if parent directory exists or can be created
	dir := filepath.Dir(dstPath)
	if dir != "." && dir != "/" {
		if info, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("parent directory '%s' does not exist", dir)
			}
			return fmt.Errorf("cannot access parent directory '%s': %v", dir, err)
		} else if !info.IsDir() {
			return fmt.Errorf("parent path '%s' is not a directory", dir)
		}
	}

	// Ensure the destination path looks like a file (has a filename)
	filename := filepath.Base(dstPath)
	if filename == "." || filename == ".." {
		return fmt.Errorf("destination path '%s' does not specify a filename", dstPath)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(receiveCmd)

	// Define flags with struct binding
	receiveCmd.Flags().StringVarP(&receiveFlags.DstPath, "dst", "d", "", "Destination path to save received file (required)")

	// Mark required flags
	receiveCmd.MarkFlagRequired("dst")

	// Bind flags to viper for environment variable support
	viper.BindPFlag("receive.dst", receiveCmd.Flags().Lookup("dst"))

	// Future flag bindings can be easily added here:
	// viper.BindPFlag("receive.verbose", receiveCmd.Flags().Lookup("verbose"))
	// viper.BindPFlag("receive.timeout", receiveCmd.Flags().Lookup("timeout"))
}

// runReceiverApp creates and runs the receiver application
func runReceiverApp(flags *ReceiveFlags) error {
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

	// Create receiver options from flags
	opts := &app.ReceiverOptions{
		DstPath: flags.DstPath,
	}

	receiverApp := app.NewReceiverApp(cfg, peerService, dataChannelService, signalingService, ui, fileService)
	return receiverApp.Run(ctx, opts)
}
