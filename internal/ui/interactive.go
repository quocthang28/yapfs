package ui

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"yapfs/internal/transport"
	"yapfs/pkg/utils"
)

// ConsoleUI implements simple console-based interactive UI
type ConsoleUI struct{}

// NewConsoleUI creates a new console-based interactive UI
func NewConsoleUI() *ConsoleUI {
	return &ConsoleUI{}
}

// ShowMessage displays a message to the user
func (c *ConsoleUI) ShowMessage(message string) {
	log.Printf("%s\n", message)
}

// InputCode prompts user to input an 8-character alphanumeric code with validation
func (c *ConsoleUI) InputCode(ctx context.Context) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		fmt.Printf("Enter code from sender: ")
		scanner.Scan()
		code := strings.TrimSpace(scanner.Text())

		if utils.IsValidCode(code) {
			return code, nil
		}

		fmt.Printf("Invalid code. Please enter again.\n")
	}
}

// UpdateProgress displays progress updates for file transfer
func (c *ConsoleUI) UpdateProgress(update transport.ProgressUpdate) {
	fmt.Printf("\rReceiving: %.1f%% (%.2f MB/s) - %s / %s",
		update.Percentage,
		update.Throughput,
		utils.FormatFileSize(int64(update.BytesSent)),
		utils.FormatFileSize(int64(update.BytesTotal)))
}
