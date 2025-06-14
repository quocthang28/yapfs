package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"yapfs/pkg/utils"
)

// ConsoleUI implements simple console-based interactive UI
type ConsoleUI struct {
	// Pure UI - no service dependencies
}

// NewConsoleUI creates a new console-based interactive UI
func NewConsoleUI() *ConsoleUI {
	return &ConsoleUI{}
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

// InputCode prompts user to input an 8-character alphanumeric code with validation
func (c *ConsoleUI) InputCode() (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("Enter 8-character alphanumeric code: ")
		scanner.Scan()
		code := strings.TrimSpace(scanner.Text())

		if utils.IsValidCode(code) {
			return strings.ToUpper(code), nil
		}

		fmt.Printf("❌ Invalid code. Please enter exactly 8 alphanumeric characters (letters and numbers only).\n")
	}
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
