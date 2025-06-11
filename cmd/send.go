// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package cmd

import (
	"log"
	"yapfs/internal/app"

	"github.com/spf13/cobra"
)

var filePath string

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
	Run: func(cmd *cobra.Command, args []string) {
		if filePath == "" {
			log.Fatal("--file flag is required")
		}
		log.Printf("Starting sender for file: %s", filePath)
		if err := runSenderApp(filePath); err != nil {
			log.Fatalf("Sender failed: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	sendCmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file to send (required)")
	sendCmd.MarkFlagRequired("file")
}

// runSenderApp creates and runs the sender application
func runSenderApp(filePath string) error {
	ctx := createContext()
	peerService, dataChannelService, signalingService, ui, fileService := createServices()

	senderApp := app.NewSenderApp(cfg, peerService, dataChannelService, signalingService, ui, fileService)
	return senderApp.RunWithFile(ctx, filePath)
}
