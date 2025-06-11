package ui

import "fmt"

// ProgressReporter defines an interface for reporting file transfer progress
type ProgressReporter interface {
	// UpdateProgress reports current transfer progress
	UpdateProgress(progress, throughput float64, bytesSent, bytesTotal int64)

	// UpdateThroughput reports just throughput (for backward compatibility)
	UpdateThroughput(mbps float64)

	// ReportError reports an error during transfer
	ReportError(err error)

	// ReportCompletion reports successful completion
	ReportCompletion(message string)
}

// ConsoleProgressReporter implements ProgressReporter for simple console UI
type ConsoleProgressReporter struct {
	ui *ConsoleUI
}

// NewConsoleProgressReporter creates a new progress reporter for console UI
func NewConsoleProgressReporter(ui *ConsoleUI) *ConsoleProgressReporter {
	return &ConsoleProgressReporter{ui: ui}
}

// UpdateProgress implements ProgressReporter interface
func (c *ConsoleProgressReporter) UpdateProgress(progress, throughput float64, bytesSent, bytesTotal int64) {
	c.ui.UpdateProgress(progress, throughput, bytesSent, bytesTotal)
}

// UpdateThroughput implements ProgressReporter interface (for backward compatibility)
func (c *ConsoleProgressReporter) UpdateThroughput(mbps float64) {
	// For backward compatibility, just log the throughput
	// Real progress updates should use UpdateProgress
}

// ReportError implements ProgressReporter interface
func (c *ConsoleProgressReporter) ReportError(err error) {
	fmt.Printf("\n❌ Transfer failed: %s\n", err.Error())
}

// ReportCompletion implements ProgressReporter interface
func (c *ConsoleProgressReporter) ReportCompletion(message string) {
	fmt.Printf("\n✅ %s\n", message)
}
