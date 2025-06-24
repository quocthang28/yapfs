package types

// FileMetadata contains information about the file being transferred
type FileMetadata struct {
	Name     string `json:"name"`     // Original filename
	Size     int64  `json:"size"`     // File size in bytes
	MimeType string `json:"mimeType"` // MIME type of the file
	Checksum string `json:"checksum"` // SHA-256 checksum
}

// ProgressUpdate represents raw file transfer progress data
type ProgressUpdate struct {
	NewBytes uint64        // New bytes transferred in this update
	MetaData *FileMetadata // This should only be sent once at the start
}
