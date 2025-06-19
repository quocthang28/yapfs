package ui

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"yapfs/internal/transport"
	"yapfs/pkg/utils"

	"github.com/schollz/progressbar/v3"
)

// ConsoleUI implements console-based interactive UI with progress tracking
type ConsoleUI struct {
	bar            *progressbar.ProgressBar
	operation      string // "Sending" or "Receiving"
	filename       string // Current file being transferred
	totalBytes     uint64
	currentBytes   uint64 // Cumulative bytes transferred
	startTime      time.Time
	lastUpdateTime time.Time
}

// NewConsoleUI creates a new console-based interactive UI
func NewConsoleUI(operation string) *ConsoleUI {
	ui := &ConsoleUI{
		operation: operation,
	}
	// Don't initialize progress bar yet - wait for first progress update
	return ui
}

// ShowMessage displays a message to the user
func (c *ConsoleUI) ShowMessage(message string) {
	log.Printf("%s\n", message)
}

// InputCode prompts user to input an 8-character alphanumeric code with validation
func (c *ConsoleUI) InputCode(ctx context.Context) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Create a channel to receive the input
		inputCh := make(chan string, 1)
		defer close(inputCh)

		fmt.Printf("Enter code from sender: ")
		go func() {
			if scanner.Scan() {
				inputCh <- strings.TrimSpace(scanner.Text())
			}
		}()

		// Wait for either input or context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case code := <-inputCh:
			if utils.IsValidCode(code) {
				return code, nil
			}
			fmt.Printf("Invalid code. Please enter again.\n")
		}
	}
}

// StartUpdatingSenderProgress starts progress tracking for sending with internal progress handling
func (c *ConsoleUI) StartUpdatingSenderProgress(ctx context.Context, progressCh <-chan transport.ProgressUpdate) {
	c.handleProgressUpdates(ctx, progressCh)
}

// StartUpdatingReceiverProgress starts progress tracking for receiving with internal progress handling
func (c *ConsoleUI) StartUpdatingReceiverProgress(ctx context.Context, progressCh <-chan transport.ProgressUpdate) {
	c.handleProgressUpdates(ctx, progressCh)
}

// initProgressBar initializes the progress bar with default settings
func (c *ConsoleUI) initProgressBar() {
	if c.bar != nil {
		return // Already initialized
	}

	// Use operation if set, otherwise use generic description
	description := "Transfer..."
	if c.operation != "" {
		description = fmt.Sprintf("%s...", c.operation)
	}

	// Create a placeholder progress bar that will be updated with actual size later
	c.bar = progressbar.NewOptions64(-1, // Indeterminate progress initially
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(false), // Disable built-in byte display, we'll customize it
		progressbar.OptionSetWidth(50),
		progressbar.OptionThrottle(200*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionUseANSICodes(true), // Enable ANSI codes for proper line clearing
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetPredictTime(false), // Disable ETA prediction
	)
}

// handleProgressUpdates processes progress updates internally
func (c *ConsoleUI) handleProgressUpdates(ctx context.Context, progressCh <-chan transport.ProgressUpdate) {
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop progress updates
			return
		case update, ok := <-progressCh:
			if !ok {
				// Channel closed, transfer complete
				return
			}
			
			c.updateProgress(update)

			// Exit when transfer finishes (completion is handled in updateProgress)
			if c.currentBytes > 0 && update.MetaData.Size > 0 && c.currentBytes >= uint64(update.MetaData.Size) {
				return
			}
		}
	}
}

// updateProgress updates the progress bar with current transfer state
func (c *ConsoleUI) updateProgress(update transport.ProgressUpdate) {
	// Initialize progress bar on first update (metadata or data)
	if c.bar == nil {
		c.initProgressBar()
	}

	// Accumulate new bytes
	c.currentBytes += update.NewBytes

	// Update total bytes and bar properties if this is the first metadata update
	if c.totalBytes == 0 && update.MetaData.Size > 0 {
		c.totalBytes = uint64(update.MetaData.Size)
		c.filename = update.MetaData.Name
		// Update progress bar with correct total size
		c.bar.ChangeMax64(int64(c.totalBytes))
		
		// Set initial description with filename
		c.bar.Describe(fmt.Sprintf("%s %s", c.operation, c.filename))
	}

	// Capture start time on first actual data chunk (not metadata)
	if c.startTime.IsZero() && update.NewBytes > 0 {
		c.startTime = time.Now()
		c.lastUpdateTime = c.startTime
	}

	now := time.Now()

	// Calculate percentage and throughput
	percentage := float64(c.currentBytes) / float64(c.totalBytes) * 100.0
	throughput := 0.0

	// Only calculate throughput if we have a valid start time and elapsed time
	if !c.startTime.IsZero() && c.currentBytes > 0 {
		elapsed := now.Sub(c.startTime)
		if elapsed.Seconds() > 0 {
			throughput = float64(c.currentBytes) / elapsed.Seconds() / (1024 * 1024) // MB/s
		}
	}

	// Smart throttling: update more frequently for small files or quick transfers
	isTinyFile := c.totalBytes < 1024       // Files smaller than 1KB
	isSmallFile := c.totalBytes < 1024*1024 // 1MB
	isQuickTransfer := false
	if !c.startTime.IsZero() {
		elapsed := now.Sub(c.startTime)
		isQuickTransfer = elapsed < 2*time.Second
	}

	updateInterval := time.Second
	if isTinyFile {
		updateInterval = 0 // Always update for tiny files
	} else if isSmallFile || isQuickTransfer {
		updateInterval = 200 * time.Millisecond
	}

	timeSinceLastUpdate := now.Sub(c.lastUpdateTime)
	isComplete := c.currentBytes >= c.totalBytes

	// Update display if enough time has passed or transfer is complete
	if timeSinceLastUpdate >= updateInterval || isComplete || percentage == 0.0 {
		// Update the progress bar with bytes sent (only if bar is initialized)
		if c.bar != nil {
			_ = c.bar.Set64(int64(c.currentBytes))
			
			// Update description with current progress and throughput using accurate formatting
			if c.totalBytes > 0 && c.filename != "" {
				currentSizeStr := utils.FormatFileSize(int64(c.currentBytes))
				totalSizeStr := utils.FormatFileSize(int64(c.totalBytes))
				throughputStr := fmt.Sprintf("%.1f MB/s", throughput)
				
				c.bar.Describe(fmt.Sprintf("%s %s (%s/%s, %s)", 
					c.operation, c.filename, currentSizeStr, totalSizeStr, throughputStr))
			}
		}

		c.lastUpdateTime = now
	}

	// Complete progress bar and show summary when transfer is complete
	if isComplete {
		// Finish the progress bar first
		c.completeProgress()

		elapsed := time.Duration(0)
		if !c.startTime.IsZero() {
			elapsed = now.Sub(c.startTime)
		}

		c.showTransferSummary(percentage, throughput, elapsed)
	}
}

// completeProgress marks the progress as complete
func (c *ConsoleUI) completeProgress() {
	if c.bar == nil {
		return
	}
	_ = c.bar.Finish()
}

// showTransferSummary displays a summary of the completed transfer
func (c *ConsoleUI) showTransferSummary(percentage, throughput float64, elapsed time.Duration) {
	fmt.Printf("\n\n=============================================\n")
	fmt.Printf("File transfer completed successfully!\n")
	fmt.Printf("+ Total bytes sent: %s\n", utils.FormatFileSize(int64(c.currentBytes)))
	fmt.Printf("+ Transfer time: %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("+ Average throughput: %.2f MB/s\n", throughput)
	fmt.Printf("+ Completion: %.1f%%\n", percentage)
	fmt.Printf("=============================================\n")
}
