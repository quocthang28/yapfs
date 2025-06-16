package transport

import (
	"time"
	"yapfs/internal/processor"
)

// ProgressUpdate represents file transfer progress information
type ProgressUpdate struct {
	BytesSent   uint64
	BytesTotal  uint64
	Percentage  float64
	Throughput  float64 // bytes/second
	ElapsedTime time.Duration
	MetaData    processor.FileMetadata
}
