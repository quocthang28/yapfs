// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package ui

import "github.com/pion/webrtc/v4"

// InteractiveUI defines the interface for user interactions
type InteractiveUI interface {
	// OutputSDP displays an SDP for the user to copy
	OutputSDP(sd webrtc.SessionDescription, sdpType string) error
	
	// InputSDP prompts the user to paste an SDP
	InputSDP(sdpType string) (webrtc.SessionDescription, error)
	
	// ShowMessage displays a message to the user
	ShowMessage(message string)
	
	// ShowInstructions displays instructions for the current operation
	ShowInstructions(role string)
	
	// WaitForUserInput waits for user confirmation before proceeding
	WaitForUserInput(prompt string)
}