package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ResolveDestinationPath resolves the destination path, validating directories
func ResolveDestinationPath(destPath string) (string, error) {
	// Check if the path exists and is a directory
	if info, err := os.Stat(destPath); err == nil {
		if info.IsDir() {
			// Valid directory - return as is (filename will come from metadata)
			return destPath, nil
		}
		// Path exists but is not a directory
		return "", fmt.Errorf("destination path '%s' exists but is not a directory", destPath)
	} else if os.IsNotExist(err) {
		// Path doesn't exist - this could be a file path or non-existent directory
		dir := filepath.Dir(destPath)
		if info, dirErr := os.Stat(dir); dirErr == nil && info.IsDir() {
			// Parent exists and is a directory - treat destPath as intended directory name
			// We'll create it when needed
			return destPath, nil
		}
		// Parent doesn't exist
		return "", fmt.Errorf("parent directory does not exist: %s", dir)
	} else {
		// Other error accessing the path
		return "", fmt.Errorf("cannot access destination path: %w", err)
	}
}