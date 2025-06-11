// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pion/webrtc/v4"
	webrtcService "yapfs/internal/webrtc"
)

// ConsoleUI implements console-based interaction
type ConsoleUI struct {
	signalingService *webrtcService.SignalingService
}

// NewConsoleUI creates a new console-based interactive UI
func NewConsoleUI(signalingService *webrtcService.SignalingService) *ConsoleUI {
	return &ConsoleUI{
		signalingService: signalingService,
	}
}

// OutputSDP displays an SDP for the user to copy
func (c *ConsoleUI) OutputSDP(sd webrtc.SessionDescription, sdpType string) error {
	encoded, err := c.signalingService.EncodeSessionDescription(sd)
	if err != nil {
		return fmt.Errorf("failed to encode SDP: %w", err)
	}

	fmt.Printf("\n%s SDP (copy this and send to the other peer):\n", sdpType)
	fmt.Println("========================================")
	fmt.Println(encoded)
	fmt.Println("========================================")
	return nil
}

// InputSDP prompts the user to paste an SDP
func (c *ConsoleUI) InputSDP(sdpType string) (webrtc.SessionDescription, error) {
	fmt.Printf("\nPaste the %s SDP from the other peer and press Enter:\n", sdpType)
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	encoded := strings.TrimSpace(scanner.Text())

	sd, err := c.signalingService.DecodeSessionDescription(encoded)
	if err != nil {
		return sd, fmt.Errorf("failed to decode SDP: %w", err)
	}

	return sd, nil
}

// ShowMessage displays a message to the user
func (c *ConsoleUI) ShowMessage(message string) {
	fmt.Println(message)
}

// ShowInstructions displays instructions for the current operation
func (c *ConsoleUI) ShowInstructions(role string) {
	switch role {
	case "sender":
		fmt.Println("YAPFS - P2P File Sharing (Sender)")
		fmt.Println("================================")
		fmt.Println("This instance will create an SDP offer and send the file to the receiver.")
		fmt.Println("Instructions:")
		fmt.Println("1. Copy the offer SDP that will be displayed")
		fmt.Println("2. Send it to the receiver instance (via any communication method)")
		fmt.Println("3. Get the answer SDP from the receiver")
		fmt.Println("4. Paste it back here when prompted")
		fmt.Println("5. File transfer will begin automatically once connected")
	case "receiver":
		fmt.Println("YAPFS - P2P File Sharing (Receiver)")
		fmt.Println("===================================")
		fmt.Println("This instance will respond to an SDP offer and receive the file from the sender.")
		fmt.Println("Instructions:")
		fmt.Println("1. Get the offer SDP from the sender instance")
		fmt.Println("2. Paste it here when prompted")
		fmt.Println("3. Copy the answer SDP that will be displayed")
		fmt.Println("4. Send it back to the sender instance")
		fmt.Println("5. File will be saved to the specified destination once received")
	default:
		fmt.Printf("YAPFS - P2P File Sharing (%s Mode)\n", strings.Title(role))
	}
	fmt.Println()
}

// WaitForUserInput waits for user confirmation before proceeding
func (c *ConsoleUI) WaitForUserInput(prompt string) {
	if prompt == "" {
		prompt = "Press Enter to continue..."
	}
	fmt.Printf("\n%s\n", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}