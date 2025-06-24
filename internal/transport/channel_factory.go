package transport

import (
	"context"
	"fmt"

	"yapfs/internal/config"

	"github.com/pion/webrtc/v4"
)

// CreateSenderChannel creates a channel configured for sending files and sets up WebRTC
func CreateSenderChannel(ctx context.Context, cfg *config.Config, peerConn *webrtc.PeerConnection, 
	label string, filePath string, onCompleted func(error)) (*Channel, error) {
	
	// Create sender handler
	handler, err := NewSenderHandler(ctx, cfg, filePath, onCompleted)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender handler: %w", err)
	}
	
	// Create generic channel with sender handler
	channel := NewChannel(ctx, cfg, handler)
	
	// Set channel reference in handler
	handler.SetChannel(channel)
	
	// Create WebRTC data channel
	err = channel.CreateDataChannel(peerConn, label)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebRTC data channel: %w", err)
	}
	
	return channel, nil
}

// CreateReceiverChannel creates a channel configured for receiving files and sets up WebRTC
func CreateReceiverChannel(ctx context.Context, cfg *config.Config, peerConn *webrtc.PeerConnection, destPath string, onCompleted func(error)) (*Channel, error) {
	// Create receiver handler
	handler := NewReceiverHandler(ctx, cfg, destPath, onCompleted)
	
	// Create generic channel with receiver handler
	channel := NewChannel(ctx, cfg, handler)
	
	// Set channel reference in handler
	handler.SetChannel(channel)
	
	// Setup to receive incoming data channel
	err := channel.SetupReceiverDataChannel(peerConn)
	if err != nil {
		return nil, fmt.Errorf("failed to setup receiver data channel: %w", err)
	}
	
	return channel, nil
}