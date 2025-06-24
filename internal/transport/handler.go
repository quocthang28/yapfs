package transport

import (
	"context"
	"yapfs/pkg/types"
)

// MessageHandler defines the interface for handling messages and channel lifecycle events
// This allows for role-specific logic (sender/receiver) while using a generic channel
type MessageHandler interface {
	// Message processing
	HandleMessage(msg Message, progressCh chan<- types.ProgressUpdate) error
	
	// Lifecycle events
	OnChannelReady() error
	OnChannelClosed()
	OnChannelError(err error)
}

// SenderState represents the current state of the sender in the transfer protocol
type SenderState int

const (
	SenderInitializing SenderState = iota
	SenderWaitingForReady
	SenderSendingMetadata
	SenderWaitingForMetadataAck
	SenderTransferringData
	SenderWaitingForCompletion
	SenderCompleted
	SenderError
)

// String returns the string representation of SenderState
func (s SenderState) String() string {
	switch s {
	case SenderInitializing:
		return "Initializing"
	case SenderWaitingForReady:
		return "WaitingForReady"
	case SenderSendingMetadata:
		return "SendingMetadata"
	case SenderWaitingForMetadataAck:
		return "WaitingForMetadataAck"
	case SenderTransferringData:
		return "TransferringData"
	case SenderWaitingForCompletion:
		return "WaitingForCompletion"
	case SenderCompleted:
		return "Completed"
	case SenderError:
		return "Error"
	default:
		return "Unknown"
	}
}

// ReceiverState represents the current state of the receiver in the transfer protocol
type ReceiverState int

const (
	ReceiverInitializing ReceiverState = iota
	ReceiverReady
	ReceiverReceivingMetadata
	ReceiverPreparingFile
	ReceiverReceivingData
	ReceiverCompleted
	ReceiverError
)

// String returns the string representation of ReceiverState
func (r ReceiverState) String() string {
	switch r {
	case ReceiverInitializing:
		return "Initializing"
	case ReceiverReady:
		return "Ready"
	case ReceiverReceivingMetadata:
		return "ReceivingMetadata"
	case ReceiverPreparingFile:
		return "PreparingFile"
	case ReceiverReceivingData:
		return "ReceivingData"
	case ReceiverCompleted:
		return "Completed"
	case ReceiverError:
		return "Error"
	default:
		return "Unknown"
	}
}

// BaseHandler provides common functionality for handlers
type BaseHandler struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewBaseHandler creates a new base handler with context
func NewBaseHandler(ctx context.Context) *BaseHandler {
	ctx, cancel := context.WithCancel(ctx)
	return &BaseHandler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context returns the handler's context
func (h *BaseHandler) Context() context.Context {
	return h.ctx
}

// Cancel cancels the handler's context
func (h *BaseHandler) Cancel() {
	if h.cancel != nil {
		h.cancel()
	}
}

// IsCancelled checks if the handler's context has been cancelled
func (h *BaseHandler) IsCancelled() bool {
	select {
	case <-h.ctx.Done():
		return true
	default:
		return false
	}
}

// OnChannelError provides default error handling
func (h *BaseHandler) OnChannelError(err error) {
	h.Cancel()
}

// OnChannelClosed provides default close handling
func (h *BaseHandler) OnChannelClosed() {
	h.Cancel()
}