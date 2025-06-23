# Generic Bidirectional Channel Design

## Problem Statement

Current architecture has separate `SenderChannel` and `ReceiverChannel` that only support unidirectional communication. With acknowledgement protocol requirements, both channels need to send and receive messages, creating code duplication and architectural blur.

## Design Goals

1. **Generic Channel**: Single channel implementation that can send/receive messages
2. **Role-based Handlers**: Pluggable handlers for sender/receiver specific logic  
3. **Protocol Support**: Support both current unidirectional and future bidirectional protocols
4. **Clean Architecture**: Separate transport concerns from application logic
5. **Backward Compatibility**: Maintain current API contracts

## Architecture Overview

### Core Components

```go
// Generic bidirectional channel
type Channel struct {
    ctx             context.Context
    config          *config.Config
    dataChannel     *webrtc.DataChannel
    dataProcessor   *processor.DataProcessor
    handler         MessageHandler
    readyCh         chan struct{}
    bufferControlCh chan struct{}
    
    // Message routing
    incomingMsgCh   chan webrtc.DataChannelMessage
    outgoingMsgCh   chan []byte
}

// Handler interface for role-specific logic
type MessageHandler interface {
    // Message processing
    HandleMessage(msg webrtc.DataChannelMessage, progressCh chan types.ProgressUpdate) error
    
    // Lifecycle events
    OnChannelReady() error
    OnChannelClosed()
    OnChannelError(err error)
    
    // Protocol negotiation
    GetProtocolVersion() string
    SupportsAcknowledgements() bool
}

// Factory for creating channels with specific handlers
type ChannelFactory struct {
    config *config.Config
}
```

### Handler Implementations

```go
// Sender-specific handler
type SenderHandler struct {
    filePath        string
    fileMetadata    *types.FileMetadata
    transferState   SenderState
    ackTimeouts     map[string]time.Duration
}

// Receiver-specific handler  
type ReceiverHandler struct {
    destPath        string
    fileMetadata    *types.FileMetadata
    transferState   ReceiverState
    metadataReceived bool
}

// Transfer states for protocol management
type SenderState int
const (
    SenderWaitingForReady = iota
    SenderSendingMetadata
    SenderWaitingForMetadataAck
    SenderTransferringData
    SenderWaitingForCompletion
    SenderCompleted
)

type ReceiverState int
const (
    ReceiverInitializing = iota
    ReceiverReady
    ReceiverReceivingMetadata
    ReceiverPreparingFile
    ReceiverReceivingData
    ReceiverCompleted
)
```

## Message Protocol Design

### Message Types

```go
type MessageType string

const (
    // Control messages
    MSG_READY            MessageType = "READY"
    MSG_METADATA_ACK     MessageType = "METADATA_ACK"
    MSG_TRANSFER_COMPLETE MessageType = "TRANSFER_COMPLETE"
    MSG_ERROR            MessageType = "ERROR"
    
    // Data messages  
    MSG_METADATA         MessageType = "METADATA"
    MSG_FILE_DATA        MessageType = "FILE_DATA"
    MSG_EOF              MessageType = "EOF"
)

type Message struct {
    Type    MessageType `json:"type"`
    Payload []byte      `json:"payload,omitempty"`
    Error   string      `json:"error,omitempty"`
}
```

### Protocol Flow

**Enhanced Bidirectional Flow:**
1. **Initialization**
   - Channel opens → Receiver sends `READY`
   - Sender receives `READY` → sends `METADATA`

2. **Metadata Exchange**
   - Receiver processes metadata → sends `METADATA_ACK` or `ERROR`
   - Sender receives `METADATA_ACK` → starts data transfer

3. **Data Transfer**
   - Sender sends `FILE_DATA` chunks
   - Receiver processes chunks (optional periodic ACKs for large files)

4. **Completion**
   - Sender sends `EOF`
   - Receiver verifies checksum → sends `TRANSFER_COMPLETE` or `ERROR`
   - Sender receives confirmation → closes gracefully

## Implementation Plan

### Phase 1: Core Channel Infrastructure

1. **Create Generic Channel** (`internal/transport/channel.go`)
   ```go
   func NewChannel(cfg *config.Config, handler MessageHandler) *Channel
   func (c *Channel) CreateDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection, label string) error
   func (c *Channel) SendMessage(msgType MessageType, payload []byte) error
   func (c *Channel) StartMessageLoop() (<-chan types.ProgressUpdate, error)
   ```

2. **Define MessageHandler Interface** (`internal/transport/handler.go`)
   - Abstract interface for message handling
   - Lifecycle management methods
   - Protocol negotiation support

### Phase 2: Handler Implementations

3. **Implement SenderHandler** (`internal/transport/sender_handler.go`)
   - Migrate logic from current `SenderChannel`
   - Add acknowledgement handling
   - Implement state machine

4. **Implement ReceiverHandler** (`internal/transport/receiver_handler.go`)
   - Migrate logic from current `ReceiverChannel` 
   - Add acknowledgement sending
   - Implement state machine

### Phase 3: Factory and Integration

5. **Create ChannelFactory** (`internal/transport/channel_factory.go`)
   ```go
   func (f *ChannelFactory) CreateSenderChannel(filePath string) (*Channel, error)
   func (f *ChannelFactory) CreateReceiverChannel(destPath string) (*Channel, error)
   ```

6. **Update DataChannelService** (`internal/transport/data_channel_service.go`)
   - Replace current sender/receiver channels with generic channel
   - Maintain existing API for backward compatibility

### Phase 4: Protocol Enhancement

7. **Implement Acknowledgement Protocol**
   - Add message serialization/deserialization
   - Implement timeout handling
   - Add retry mechanisms

8. **Add Protocol Versioning**
   - Support both legacy and enhanced protocols
   - Graceful fallback for compatibility

## Migration Strategy

### Backward Compatibility

```go
// Maintain existing DataChannelService API
type DataChannelService struct {
    config  *config.Config
    factory *ChannelFactory
    channel *Channel
}

// Existing methods delegate to generic channel
func (s *DataChannelService) CreateFileSenderDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection, label string, filePath string) error {
    handler := NewSenderHandler(s.config, filePath)
    s.channel = s.factory.CreateChannel(handler)
    return s.channel.CreateDataChannel(ctx, peerConn, label)
}

func (s *DataChannelService) SendFile() (<-chan types.ProgressUpdate, error) {
    return s.channel.StartMessageLoop()
}
```

### Testing Strategy

1. **Unit Tests**: Test each handler independently
2. **Integration Tests**: Test full protocol flows
3. **Compatibility Tests**: Ensure legacy protocol still works
4. **Error Handling Tests**: Test timeout and error scenarios

## Benefits

1. **Reduced Code Duplication**: Single channel implementation
2. **Enhanced Reliability**: Proper acknowledgement protocol
3. **Better Error Handling**: Bidirectional error reporting
4. **Extensibility**: Easy to add new message types/protocols
5. **Clean Architecture**: Separation of transport and application logic
6. **Future-Proof**: Foundation for advanced features like resume, multiple files, etc.

## File Structure

```
internal/transport/
├── channel.go              # Generic Channel implementation
├── handler.go              # MessageHandler interface
├── sender_handler.go       # Sender-specific logic
├── receiver_handler.go     # Receiver-specific logic
├── channel_factory.go      # Factory for creating channels
├── message.go              # Message types and protocol
├── data_channel_service.go # Updated service (backward compatibility)
├── sender_channel.go       # [DEPRECATED] Current sender
└── receiver_channel.go     # [DEPRECATED] Current receiver
```

This design provides a clean foundation for bidirectional communication while maintaining the current API and adding robust acknowledgement support.