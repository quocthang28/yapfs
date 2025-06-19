package transport

import (
	"yapfs/internal/processor"
)

// ProgressUpdate represents raw file transfer progress data
type ProgressUpdate struct {
	NewBytes uint64 // New bytes transferred in this update
	MetaData processor.FileMetadata
}
