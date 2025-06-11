package config

import (
	"errors"
	"github.com/pion/webrtc/v4"
)

var (
	ErrInvalidBufferConfig   = errors.New("buffered amount low threshold must be less than max buffered amount")
	ErrInvalidPacketSize     = errors.New("packet size must be greater than 0")
	ErrInvalidReportInterval = errors.New("throughput report interval must be greater than 0")
)

// Config holds all application configuration
type Config struct {
	WebRTC WebRTCConfig `yaml:"webrtc"`
}

// WebRTCConfig holds WebRTC-specific configuration
type WebRTCConfig struct {
	ICEServers              []webrtc.ICEServer `yaml:"ice_servers"`
	BufferedAmountLowThreshold uint64           `yaml:"buffered_amount_low_threshold"`
	MaxBufferedAmount       uint64             `yaml:"max_buffered_amount"`
	PacketSize              int                `yaml:"packet_size"`
	ThroughputReportInterval int               `yaml:"throughput_report_interval_ms"`
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
			ThroughputReportInterval:   1000,        // 1 second
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
	if c.WebRTC.ThroughputReportInterval <= 0 {
		return ErrInvalidReportInterval
	}
	return nil
}