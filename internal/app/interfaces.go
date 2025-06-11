// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package app

import "context"

// SenderApp defines the interface for sender application logic
type SenderApp interface {
	// Run starts the sender application
	Run(ctx context.Context) error
	// RunWithFile starts the sender application to send a specific file
	RunWithFile(ctx context.Context, filePath string) error
}

// ReceiverApp defines the interface for receiver application logic  
type ReceiverApp interface {
	// Run starts the receiver application
	Run(ctx context.Context) error
	// RunWithDest starts the receiver application to save file to destination
	RunWithDest(ctx context.Context, dstPath string) error
}