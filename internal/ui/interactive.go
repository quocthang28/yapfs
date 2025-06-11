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
	// Break the base64 string into 80-character lines for easier copying
	c.printWrappedBase64(encoded)
	fmt.Println("========================================")
	return nil
}

// InputSDP prompts the user to paste an SDP
func (c *ConsoleUI) InputSDP(sdpType string) (webrtc.SessionDescription, error) {
	fmt.Printf("\nPaste the %s SDP from the other peer (multiple lines, press Enter after last line, then Ctrl+D or type 'END'):\n", sdpType)
	fmt.Print("> ")

	var lines []string
	scanner := bufio.NewScanner(os.Stdin)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "END" {
			break
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("failed to read input: %w", err)
	}
	
	if len(lines) == 0 {
		return webrtc.SessionDescription{}, fmt.Errorf("no SDP input received")
	}
	
	// Combine all lines into single base64 string
	encoded := strings.Join(lines, "")
	
	sd, err := c.signalingService.DecodeSessionDescription(encoded)
	if err != nil {
		return sd, fmt.Errorf("failed to decode SDP: %w", err)
	}

	return sd, nil
}

// printWrappedBase64 prints a base64 string with line breaks for better readability
func (c *ConsoleUI) printWrappedBase64(encoded string) {
	const lineLength = 80
	for i := 0; i < len(encoded); i += lineLength {
		end := min(i+lineLength, len(encoded))
		fmt.Println(encoded[i:end])
	}
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
		fmt.Println("1. Copy ALL lines of the offer SDP that will be displayed")
		fmt.Println("2. Send it to the receiver instance (via any communication method)")
		fmt.Println("3. Get the answer SDP from the receiver (also multiple lines)")
		fmt.Println("4. Paste ALL lines back here when prompted, then type 'END' or press Ctrl+D")
		fmt.Println("5. File transfer will begin automatically once connected")
	case "receiver":
		fmt.Println("YAPFS - P2P File Sharing (Receiver)")
		fmt.Println("===================================")
		fmt.Println("This instance will respond to an SDP offer and receive the file from the sender.")
		fmt.Println("Instructions:")
		fmt.Println("1. Get the offer SDP from the sender instance (multiple lines)")
		fmt.Println("2. Paste ALL lines here when prompted, then type 'END' or press Ctrl+D")
		fmt.Println("3. Copy ALL lines of the answer SDP that will be displayed")
		fmt.Println("4. Send it back to the sender instance")
		fmt.Println("5. File will be saved to the specified destination once received")
	default:
		fmt.Printf("YAPFS - P2P File Sharing (%s Mode)\n", strings.ToUpper(role[:1])+role[1:])
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