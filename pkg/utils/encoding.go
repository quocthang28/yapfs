package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Encode encodes any value to JSON and then to base64
func Encode[T any](value T) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal value: %w", err)
	}
	
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// Decode decodes a base64 encoded JSON string to the specified type
func Decode[T any](encoded string) (T, error) {
	var result T

	if encoded == "" {
		return result, fmt.Errorf("encoded string is empty")
	}

	bytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return result, fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(bytes) == 0 {
		return result, fmt.Errorf("decoded bytes are empty")
	}

	err = json.Unmarshal(bytes, &result)
	if err != nil {
		return result, fmt.Errorf("failed to unmarshal JSON (bytes length: %d): %w", len(bytes), err)
	}

	return result, nil
}

// EncodeJSON encodes any value to JSON bytes
func EncodeJSON[T any](value T) ([]byte, error) {
	return json.Marshal(value)
}

// DecodeJSON decodes JSON bytes to the specified type
func DecodeJSON[T any](data []byte) (T, error) {
	var result T
	if len(data) == 0 {
		return result, fmt.Errorf("JSON data is empty")
	}

	err := json.Unmarshal(data, &result)
	if err != nil {
		return result, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return result, nil
}