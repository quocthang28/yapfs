package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConsoleUI implements simple console-based interactive UI
type ConsoleUI struct {
	// Pure UI - no service dependencies
}

// NewConsoleUI creates a new console-based interactive UI
func NewConsoleUI() *ConsoleUI {
	return &ConsoleUI{}
}

// ShowInstructions displays initial instructions for the given role
func (c *ConsoleUI) ShowInstructions(role string) {
	fmt.Printf("\n=== YAPFS - P2P File Sharing ===\n\n")

	if role == "sender" {
		fmt.Printf("This instance will create an SDP offer and send the file.\n\n")
		fmt.Printf("Instructions:\n")
		fmt.Printf("1. Offer SDP will be generated and displayed\n")
		fmt.Printf("2. Share the SDP with the receiver (via file or copy/paste)\n")
		fmt.Printf("3. Enter the answer SDP from receiver\n")
		fmt.Printf("4. File transfer will begin automatically\n\n")
	} else {
		fmt.Printf("This instance will respond to an SDP offer and receive the file.\n\n")
		fmt.Printf("Instructions:\n")
		fmt.Printf("1. Enter the offer SDP from sender\n")
		fmt.Printf("2. Answer SDP will be generated and displayed\n")
		fmt.Printf("3. Share the answer SDP back to sender (via file or copy/paste)\n")
		fmt.Printf("4. File will be saved to destination\n\n")
	}

	fmt.Printf("Press Enter to continue...")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}

// OutputSDP displays the generated SDP for sharing
func (c *ConsoleUI) OutputSDP(encoded string, sdpType string) error {

	// Save to file
	filename := fmt.Sprintf("%s_sdp.txt", strings.ToLower(sdpType))
	err := os.WriteFile(filename, []byte(encoded), 0644)
	if err != nil {
		return fmt.Errorf("failed to write SDP to file: %w", err)
	}

	fmt.Printf("\n=== %s SDP Generated ===\n\n", sdpType)
	fmt.Printf("✓ %s SDP saved to: %s\n", sdpType, filename)
	fmt.Printf("✓ %s SDP size: %d characters\n\n", sdpType, len(encoded))

	fmt.Printf("Share this SDP with the other peer:\n\n")

	// Show truncated SDP for reference
	displaySDP := encoded
	if len(displaySDP) > 200 {
		displaySDP = displaySDP[:200] + "..."
	}
	fmt.Printf("%s\n\n", displaySDP)

	fmt.Printf("Press Enter to continue to next step...")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	return nil
}

// InputSDP prompts user to input SDP and returns the raw SDP string
func (c *ConsoleUI) InputSDP(sdpType string) (string, error) {
	fmt.Printf("\n=== Enter %s SDP ===\n\n", sdpType)

	filename := fmt.Sprintf("%s_sdp.txt", strings.ToLower(sdpType))

	fmt.Printf("Options to provide SDP:\n\n")
	fmt.Printf("1. 📁 File input (recommended): ")

	// Check if file exists
	if _, err := os.Stat(filename); err == nil {
		fmt.Printf("✅ File detected\n")
	} else {
		fmt.Printf("❌ File not found\n")
	}

	fmt.Printf("   Place SDP content in '%s'\n\n", filename)
	fmt.Printf("2. Manual input: Type SDP content and press Enter\n\n")

	fmt.Printf("Press Enter to process SDP...")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	// Try file input first
	if content, err := os.ReadFile(filename); err == nil {
		cleaned := cleanSDPContent(string(content))
		if len(cleaned) > 0 {
			fmt.Printf("✓ Successfully loaded SDP from file\n")
			return cleaned, nil
		}
	}

	// Fall back to manual input
	fmt.Printf("\nNo valid SDP found in file. Please enter SDP manually:\n")
	fmt.Printf("(Paste SDP content and press Enter)\n> ")

	scanner.Scan()
	input := scanner.Text()

	if input == "" {
		return "", fmt.Errorf("no SDP provided")
	}

	cleaned := cleanSDPContent(input)
	if len(cleaned) == 0 {
		return "", fmt.Errorf("invalid SDP format")
	}

	fmt.Printf("✓ Successfully parsed SDP\n")
	return cleaned, nil
}

// ShowMessage displays a message to the user
func (c *ConsoleUI) ShowMessage(message string) {
	fmt.Printf("ℹ️  %s\n", message)
}

// WaitForUserInput waits for user to press Enter
func (c *ConsoleUI) WaitForUserInput(prompt string) {
	if prompt == "" {
		prompt = "Press Enter to continue..."
	}
	fmt.Printf("\n%s\n", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
}

// UpdateProgress displays current transfer progress
// TODO: When implementing progress reporting, use DataProcessor.FormatFileSize instead of formatBytes
func (c *ConsoleUI) UpdateProgress(progress, throughput float64, bytesSent, bytesTotal int64) {
	// Create a simple progress bar
	barWidth := 50
	filled := int(progress * float64(barWidth) / 100)
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	// TODO: Replace with DataProcessor.FormatFileSize when implementing progress updates
	fmt.Printf("\r[%s] %.1f%% | %.2f Mbps | %s / %s",
		bar, progress, throughput, "N/A", "N/A")
}

// Helper functions
func cleanSDPContent(content string) string {
	var cleanedLines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 0 && isValidBase64Line(line) {
			cleanedLines = append(cleanedLines, line)
		}
	}
	return strings.Join(cleanedLines, "")
}

func isValidBase64Line(line string) bool {
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

