package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/pion/webrtc/v4"
)

// EncodeSessionDescription encodes a session description to base64
func EncodeSessionDescription(sd webrtc.SessionDescription) (string, error) {
	bytes, err := json.Marshal(sd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal session description: %w", err)
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// DecodeSessionDescription decodes a base64 encoded session description
func DecodeSessionDescription(encoded string) (webrtc.SessionDescription, error) {
	var sd webrtc.SessionDescription

	if encoded == "" {
		return sd, fmt.Errorf("encoded session description is empty")
	}

	bytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return sd, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(bytes) == 0 {
		return sd, fmt.Errorf("decoded bytes are empty")
	}

	err = json.Unmarshal(bytes, &sd)
	if err != nil {
		return sd, fmt.Errorf("failed to unmarshal session description (bytes length: %d): %w", len(bytes), err)
	}

	return sd, nil
}