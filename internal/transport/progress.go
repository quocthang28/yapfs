package transport

import (
	"yapfs/internal/processor"
)

// ProgressUpdate represents raw file transfer progress data
type ProgressUpdate struct {
	BytesSent uint64
	MetaData  processor.FileMetadata
}
