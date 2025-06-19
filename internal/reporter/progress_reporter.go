package reporter

import (
	"context"
	"fmt"
	"log"
	"time"
	"yapfs/pkg/types"
)

// ConsoleUI implements console-based interactive UI with progress tracking
type ProgressReporter struct{}

// NewConsoleUI creates a new console-based interactive UI
func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{}
}

// InputCode prompts user to input an 8-character alphanumeric code with validation

// StartUpdatingProgress starts progress tracking for file transfer
func (pr *ProgressReporter) StartUpdatingProgress(ctx context.Context, progressCh <-chan types.ProgressUpdate) {
	var totalSize int64
	var transferredBytes uint64
	var metadata *types.FileMetadata
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			log.Println("Progress reporting stopped: user cancelled")
			return
		case progress, ok := <-progressCh:
			if !ok {
				// Channel closed - transfer complete
				if metadata != nil {
					fmt.Printf("\r%60s\r", "")
					fmt.Println("=========================================================")
					fmt.Printf("File transfer complete!\n")
					fmt.Printf("Duration: %.2f seconds\n", time.Since(startTime).Seconds())
					fmt.Printf("File: %s\n", metadata.Name)
					fmt.Printf("Total size: %d bytes\n", totalSize)
					fmt.Printf("Checksum: %s\n", metadata.Checksum)
					fmt.Println("=========================================================")
				}
				return
			}

			// First progress update contains metadata
			if progress.MetaData != nil && metadata == nil {
				metadata = progress.MetaData
				totalSize = metadata.Size
				log.Printf("Starting transfer: %s (%d bytes)\n", metadata.Name, totalSize)
				continue
			}

			// Update transferred bytes
			transferredBytes += progress.NewBytes

			// Calculate and display progress
			percent := float64(transferredBytes) / float64(totalSize) * 100
			fmt.Printf("\rProgress: %d/%d bytes (%.1f%%)\r", transferredBytes, totalSize, percent)
		}
	}
}
