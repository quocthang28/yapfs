package ui

import (
	"fmt"
	"os"
	"time"

	"yapfs/internal/transport"
	"yapfs/pkg/utils"

	"github.com/schollz/progressbar/v3"
)

// ProgressUI handles progress display for file transfers
type ProgressUI struct {
	bar       *progressbar.ProgressBar
	operation string // "Sending" or "Receiving"
}

// NewProgressUI creates a new progress UI
func NewProgressUI() *ProgressUI {
	return &ProgressUI{}
}

// startProgress initializes the progress bar for a file transfer
func (p *ProgressUI) startProgress(operation, filename string, totalBytes uint64) {
	p.operation = operation
	p.bar = progressbar.NewOptions64(int64(totalBytes),
		progressbar.OptionSetDescription(fmt.Sprintf("%s %s", operation, filename)),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(50),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetPredictTime(false),
	)
}

// StartProgressSending initializes the progress bar for sending a file
func (p *ProgressUI) StartProgressSending(filename string, totalBytes uint64) {
	p.startProgress("Sending", filename, totalBytes)
}

// StartProgressReceiving initializes the progress bar for receiving a file
func (p *ProgressUI) StartProgressReceiving(filename string, totalBytes uint64) {
	p.startProgress("Receiving", filename, totalBytes)
}

// UpdateProgress updates the progress bar with current transfer state
func (p *ProgressUI) UpdateProgress(update transport.ProgressUpdate) {
	if p.bar == nil {
		return
	}

	// Update the progress bar with bytes sent
	_ = p.bar.Set64(int64(update.BytesSent))

	// Update the description with throughput information
	throughputStr := fmt.Sprintf("%.2f MB/s", update.Throughput)
	p.bar.Describe(fmt.Sprintf("%s (%.1f%% - %s)", p.operation, update.Percentage, throughputStr))

	// Show summary only when transfer is complete
	if update.Percentage >= 100.0 {
		p.ShowTransferSummary(update)
	}
}

// CompleteProgress marks the progress as complete
func (p *ProgressUI) CompleteProgress() {
	if p.bar == nil {
		return
	}
	_ = p.bar.Finish()
}

// ShowTransferSummary displays a summary of the completed transfer
func (p *ProgressUI) ShowTransferSummary(update transport.ProgressUpdate) {
	fmt.Printf("=============================================\n")
	fmt.Printf("File transfer completed successfully!\n")
	fmt.Printf("+ Total bytes sent: %s\n", utils.FormatFileSize(int64(update.BytesSent)))
	fmt.Printf("+ Transfer time: %s\n", update.ElapsedTime.Round(time.Millisecond))
	fmt.Printf("+ Average throughput: %.2f MB/s\n", update.Throughput)
	fmt.Printf("+ Completion: %.1f%%\n", update.Percentage)
	fmt.Printf("=============================================\n")
}
