package ui

import (
	"fmt"
	"os"
	"time"

	"yapfs/internal/processor"
	"yapfs/pkg/utils"

	"github.com/schollz/progressbar/v3"
)

// ProgressUI handles progress display for file transfers
type ProgressUI struct {
	bar *progressbar.ProgressBar
}

// NewProgressUI creates a new progress UI
func NewProgressUI() *ProgressUI {
	return &ProgressUI{}
}

// StartProgress initializes the progress bar for a file transfer
func (p *ProgressUI) StartProgress(filename string, totalBytes uint64) {
	p.bar = progressbar.NewOptions64(int64(totalBytes),
		progressbar.OptionSetDescription(fmt.Sprintf("Transferring %s", filename)),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(50),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Println("\n✅ Transfer completed!")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)
}

// StartProgressReceiving initializes the progress bar for receiving a file
func (p *ProgressUI) StartProgressReceiving(filename string, totalBytes uint64) {
	p.bar = progressbar.DefaultBytes(
		int64(totalBytes),
		fmt.Sprintf("Receiving %s", filename),
	)
}

// StartProgressSending initializes the progress bar for sending a file
func (p *ProgressUI) StartProgressSending(filename string, totalBytes uint64) {
	p.bar = progressbar.DefaultBytes(
		int64(totalBytes),
		fmt.Sprintf("Sending %s", filename),
	)
}

// GetWriter returns the progressbar as an io.Writer for automatic progress tracking
func (p *ProgressUI) GetWriter() *progressbar.ProgressBar {
	return p.bar
}

// UpdateProgress updates the progress bar with current transfer state
func (p *ProgressUI) UpdateProgress(update processor.ProgressUpdate) {
	if p.bar == nil {
		return
	}

	// Update the progress bar with bytes sent
	p.bar.Set64(int64(update.BytesSent))

	// Update the description with throughput information
	throughputStr := fmt.Sprintf("%.2f MB/s", update.Throughput)
	p.bar.Describe(fmt.Sprintf("Sending (%.1f%% - %s)", update.Percentage, throughputStr))
}

// CompleteProgress marks the progress as complete
func (p *ProgressUI) CompleteProgress() {
	if p.bar == nil {
		return
	}
	p.bar.Finish()
}

// ShowTransferSummary displays a summary of the completed transfer
func (p *ProgressUI) ShowTransferSummary(update processor.ProgressUpdate) {
	fmt.Printf("\n🎉 File transfer completed successfully!\n")
	fmt.Printf("📊 Transfer Statistics:\n")
	fmt.Printf("   • Total bytes sent: %s\n", utils.FormatFileSize(int64(update.BytesSent)))
	fmt.Printf("   • Transfer time: %s\n", update.ElapsedTime.Round(time.Millisecond))
	fmt.Printf("   • Average throughput: %.2f MB/s\n", update.Throughput)
	fmt.Printf("   • Completion: %.1f%%\n", update.Percentage)
}
