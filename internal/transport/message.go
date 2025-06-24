package transport

import (
	"encoding/json"
	"fmt"
)

// MessageType represents the type of message being sent over the data channel
type MessageType string

const (
	// Control messages
	MSG_READY             MessageType = "READY"
	MSG_METADATA_ACK      MessageType = "METADATA_ACK"
	MSG_TRANSFER_COMPLETE MessageType = "TRANSFER_COMPLETE"
	MSG_ERROR             MessageType = "ERROR"

	// Data messages
	MSG_METADATA  MessageType = "METADATA"
	MSG_FILE_DATA MessageType = "FILE_DATA"
	MSG_EOF       MessageType = "EOF"
)

// Message represents a structured message sent over the data channel
type Message struct {
	Type    MessageType `json:"type"`
	Payload []byte      `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SerializeMessage converts a Message to bytes for transmission
func SerializeMessage(msg Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}
	return data, nil
}

// DeserializeMessage converts bytes back to a Message
func DeserializeMessage(data []byte) (Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	if err != nil {
		return Message{}, fmt.Errorf("failed to deserialize message: %w", err)
	}
	return msg, nil
}