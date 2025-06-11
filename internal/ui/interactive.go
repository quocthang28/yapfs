// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package ui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

	// Save to file to bypass shell character limits
	filename := fmt.Sprintf("%s_sdp.txt", strings.ToLower(sdpType))
	filePath := filepath.Join(".", filename)
	
	err = os.WriteFile(filePath, []byte(encoded), 0644)
	if err != nil {
		return fmt.Errorf("failed to write SDP to file: %w", err)
	}

	fmt.Printf("\n%s SDP:\n", sdpType)
	fmt.Println("========================================")
	fmt.Printf("✓ Saved to file: %s\n", filename)
	fmt.Printf("✓ File size: %d characters\n", len(encoded))
	fmt.Println("========================================")
	fmt.Printf("Options to share:\n")
	fmt.Printf("1. Copy from file: cat %s\n", filename)
	fmt.Printf("2. Direct copy (if your shell supports it):\n")
	fmt.Println(encoded)
	fmt.Println("========================================")
	return nil
}

// InputSDP prompts the user to paste an SDP
func (c *ConsoleUI) InputSDP(sdpType string) (webrtc.SessionDescription, error) {
	expectedFilename := fmt.Sprintf("%s_sdp.txt", strings.ToLower(sdpType))
	
	fmt.Printf("\nOptions to input %s SDP:\n", sdpType)
	fmt.Println("========================================")
	fmt.Printf("1. From file (recommended): Place SDP in '%s' and press Enter\n", expectedFilename)
	fmt.Printf("2. Manual paste: Type/paste SDP directly, then type 'END'\n")
	fmt.Println("========================================")
	fmt.Print("Choose option (1 for file, 2 for manual, or just press Enter for file): ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())
	
	// Default to file option if empty or "1"
	if choice == "" || choice == "1" {
		return c.inputSDPFromFile(expectedFilename)
	}
	
	// Manual input option
	return c.inputSDPManually(sdpType)
}

// inputSDPFromFile reads SDP from a file
func (c *ConsoleUI) inputSDPFromFile(filename string) (webrtc.SessionDescription, error) {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Printf("File '%s' not found. Please create it with the SDP content and try again.\n", filename)
		return webrtc.SessionDescription{}, fmt.Errorf("SDP file not found: %s", filename)
	}
	
	// Read SDP from file
	content, err := os.ReadFile(filename)
	if err != nil {
		return webrtc.SessionDescription{}, fmt.Errorf("failed to read SDP file: %w", err)
	}
	
	// Clean up the content - extract only valid base64 characters
	rawContent := string(content)
	var cleanedLines []string
	
	for _, line := range strings.Split(rawContent, "\n") {
		line = strings.TrimSpace(line)
		// Skip empty lines and lines that don't look like base64
		if len(line) == 0 || strings.Contains(line, "No newline") || strings.Contains(line, "file") {
			continue
		}
		// Only include lines that contain valid base64 characters
		if c.isValidBase64Line(line) {
			cleanedLines = append(cleanedLines, line)
		}
	}
	
	encoded := strings.Join(cleanedLines, "")
	if len(encoded) == 0 {
		return webrtc.SessionDescription{}, fmt.Errorf("no valid base64 content found in SDP file")
	}
	
	fmt.Printf("✓ Read SDP from file '%s' (%d characters after cleanup)\n", filename, len(encoded))
	
	sd, err := c.signalingService.DecodeSessionDescription(encoded)
	if err != nil {
		return sd, fmt.Errorf("failed to decode SDP from file: %w", err)
	}
	
	return sd, nil
}

// isValidBase64Line checks if a line contains only valid base64 characters
func (c *ConsoleUI) isValidBase64Line(line string) bool {
	if len(line) == 0 {
		return false
	}
	
	for _, char := range line {
		if !((char >= 'A' && char <= 'Z') || 
			 (char >= 'a' && char <= 'z') || 
			 (char >= '0' && char <= '9') || 
			 char == '+' || char == '/' || char == '=') {
			return false
		}
	}
	return true
}

// inputSDPManually handles manual SDP input
func (c *ConsoleUI) inputSDPManually(sdpType string) (webrtc.SessionDescription, error) {
	fmt.Printf("Paste the %s SDP (multiple lines allowed, type 'END' when done):\n", sdpType)
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
		fmt.Println("1. Offer SDP will be saved to 'offer_sdp.txt' and displayed")
		fmt.Println("2. Send the file content to receiver (via any communication method)")
		fmt.Println("3. Get the answer SDP from receiver and save it as 'answer_sdp.txt'")
		fmt.Println("4. When prompted, choose file input option (recommended) or manual paste")
		fmt.Println("5. File transfer will begin automatically once connected")
		fmt.Println("Note: File-based SDP exchange avoids shell character limits")
	case "receiver":
		fmt.Println("YAPFS - P2P File Sharing (Receiver)")
		fmt.Println("===================================")
		fmt.Println("This instance will respond to an SDP offer and receive the file from the sender.")
		fmt.Println("Instructions:")
		fmt.Println("1. Get the offer SDP from sender and save it as 'offer_sdp.txt'")
		fmt.Println("2. When prompted, choose file input option (recommended) or manual paste")
		fmt.Println("3. Answer SDP will be saved to 'answer_sdp.txt' and displayed")
		fmt.Println("4. Send the answer file content back to the sender")
		fmt.Println("5. File will be saved to the specified destination once received")
		fmt.Println("Note: File-based SDP exchange avoids shell character limits")
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