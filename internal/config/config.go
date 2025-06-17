package config

import (
	"errors"
	"github.com/pion/webrtc/v4"
)

var (
	ErrInvalidBufferConfig        = errors.New("buffered amount low threshold must be less than max buffered amount")
	ErrInvalidPacketSize          = errors.New("packet size must be greater than 0")
	ErrInvalidFirebaseConfig      = errors.New("Firebase credentials path must be set")
	ErrInvalidFirebaseProjectID   = errors.New("Firebase project ID must be set")
	ErrInvalidFirebaseDatabaseURL = errors.New("Firebase database URL must be set")
)

// Config holds all application configuration
type Config struct {
	WebRTC   WebRTCConfig   `json:"webrtc"`
	Firebase FirebaseConfig `json:"firebase"`
}

// WebRTCConfig holds WebRTC-specific configuration
type WebRTCConfig struct {
	ICEServers                 []webrtc.ICEServer `json:"ice_servers"`
	BufferedAmountLowThreshold uint64             `json:"buffered_amount_low_threshold"`
	MaxBufferedAmount          uint64             `json:"max_buffered_amount"`
	PacketSize                 int                `json:"packet_size"`
}

// FirebaseConfig holds Firebase client configuration
type FirebaseConfig struct {
	ProjectID       string `json:"project_id"`
	DatabaseURL     string `json:"database_url"`
	CredentialsPath string `json:"credentials_path"`
}

// NewDefaultConfig returns a configuration with sensible defaults
func NewDefaultConfig() *Config {
	return &Config{
		WebRTC: WebRTCConfig{
			ICEServers: []webrtc.ICEServer{
				{
					URLs: []string{"stun:stun.l.google.com:19302"},
				},
			},
			BufferedAmountLowThreshold: 512 * 1024,  // 512 KB
			MaxBufferedAmount:          1024 * 1024, // 1 MB
			PacketSize:                 1024,        // 1 KB packets
		},
		Firebase: FirebaseConfig{
			ProjectID:       "",
			DatabaseURL:     "",
			CredentialsPath: "",
		},
	}
}

// Validate ensures the configuration is valid
func (c *Config) Validate() error {
	if c.WebRTC.BufferedAmountLowThreshold >= c.WebRTC.MaxBufferedAmount {
		return ErrInvalidBufferConfig
	}
	if c.WebRTC.PacketSize <= 0 {
		return ErrInvalidPacketSize
	}
	if c.Firebase.CredentialsPath == "" {
		return ErrInvalidFirebaseConfig
	}
	if c.Firebase.ProjectID == "" {
		return ErrInvalidFirebaseProjectID
	}
	if c.Firebase.DatabaseURL == "" {
		return ErrInvalidFirebaseDatabaseURL
	}
	return nil
}
