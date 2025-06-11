// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"yapfs/internal/app"

	"github.com/spf13/cobra"
)

var dstPath string

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
	Run: func(cmd *cobra.Command, args []string) {
		if dstPath == "" {
			log.Fatal("--dst flag is required")
		}
		
		// Validate dst path
		if err := validateDstPath(dstPath); err != nil {
			log.Fatalf("Invalid destination path: %v", err)
		}
		
		log.Printf("Starting receiver, will save to: %s", dstPath)
		if err := runReceiverApp(dstPath); err != nil {
			log.Fatalf("Receiver failed: %v", err)
		}
	},
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
	receiveCmd.Flags().StringVarP(&dstPath, "dst", "d", "", "Destination path to save received file (required)")
	receiveCmd.MarkFlagRequired("dst")
}

// runReceiverApp creates and runs the receiver application
func runReceiverApp(dstPath string) error {
	ctx := createContext()
	peerService, dataChannelService, signalingService, ui, fileService := createServices()

	receiverApp := app.NewReceiverApp(cfg, peerService, dataChannelService, signalingService, ui, fileService)
	return receiverApp.RunWithDest(ctx, dstPath)
}
