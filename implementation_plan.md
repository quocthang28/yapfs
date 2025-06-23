# Implementation Plan: Generic Channel Refactoring

## Overview

This document outlines the step-by-step plan to refactor YAPFS from separate sender/receiver channels to a generic bidirectional channel architecture with pluggable handlers.

## Goals

1. **Unified Architecture**: Replace `SenderChannel` and `ReceiverChannel` with generic `Channel`
2. **Bidirectional Communication**: Enable acknowledgement protocol  
3. **Backward Compatibility**: Maintain existing API contracts
4. **Zero Downtime**: Incremental migration without breaking existing functionality
5. **Enhanced Reliability**: Proper error handling and acknowledgements

## Current State Analysis

### Existing Files
- `internal/transport/sender_channel.go` (247 lines)
- `internal/transport/receiver_channel.go` (178 lines)  
- `internal/transport/data_channel_service.go` (referenced in app layer)

### Dependencies  
- `internal/app/sender.go` (uses DataChannelService)
- `internal/app/receiver.go` (uses DataChannelService)
- `internal/processor/data_processor.go` (used by both channels)

## Implementation Phases

### Phase 1: Foundation (Week 1)

#### 1.1 Create Core Interfaces
**Files to create:**
- `internal/transport/message.go` - Message types and protocol definitions
- `internal/transport/handler.go` - MessageHandler interface
- `internal/transport/channel.go` - Generic Channel implementation

**Key tasks:**
- [ ] Define Message struct and MessageType enums
- [ ] Implement MessageHandler interface
- [ ] Create BaseHandler with common functionality
- [ ] Implement basic Channel struct with WebRTC integration
- [ ] Add message serialization/deserialization

#### 1.2 Testing Infrastructure
**Files to create:**
- `internal/transport/message_test.go`
- `internal/transport/channel_test.go`
- `internal/transport/handler_test_utils.go`

**Key tasks:**
- [ ] Unit tests for message serialization
- [ ] Mock implementations for testing
- [ ] Test utilities for handler testing

### Phase 2: Handler Implementations (Week 2)

#### 2.1 Sender Handler
**Files to create:**
- `internal/transport/sender_handler.go`

**Migration strategy:**
```go
// Migrate logic from SenderChannel to SenderHandler
type SenderHandler struct {
    *BaseHandler
    filePath      string
    fileMetadata  *types.FileMetadata
    // ... other fields from SenderChannel
}

// Key methods to implement:
func (s *SenderHandler) HandleMessage(ctx context.Context, msg Message, progressCh chan<- types.ProgressUpdate) error
func (s *SenderHandler) OnChannelReady(ctx context.Context) error
```

**Key tasks:**
- [ ] Migrate file preparation logic from `SenderChannel.CreateFileSenderDataChannel()`
- [ ] Implement acknowledgement handling for READY, METADATA_ACK, TRANSFER_COMPLETE
- [ ] Migrate data sending logic from `SenderChannel.sendFileDataPhase()`
- [ ] Add state machine for sender protocol flow
- [ ] Implement timeout handling for acknowledgements

#### 2.2 Receiver Handler  
**Files to create:**
- `internal/transport/receiver_handler.go`

**Migration strategy:**
```go
// Migrate logic from ReceiverChannel to ReceiverHandler
type ReceiverHandler struct {
    *BaseHandler
    destPath         string
    metadataReceived bool
    // ... other fields from ReceiverChannel
}
```

**Key tasks:**
- [ ] Migrate message handling from `ReceiverChannel.handleMessage()`
- [ ] Implement acknowledgement sending (READY, METADATA_ACK, TRANSFER_COMPLETE)
- [ ] Migrate file receiving logic from `ReceiverChannel.handleFileDataMessage()`
- [ ] Add checksum verification and completion reporting
- [ ] Implement error reporting to sender

### Phase 3: Channel Factory and Integration (Week 3)

#### 3.1 Channel Factory
**Files to create:**
- `internal/transport/channel_factory.go`

```go
type ChannelFactory struct {
    config *config.Config
}

func (f *ChannelFactory) CreateSenderChannel(filePath string) (*Channel, error) {
    handler := NewSenderHandler(f.config, filePath)
    return NewChannel(f.config, handler), nil
}

func (f *ChannelFactory) CreateReceiverChannel(destPath string) (*Channel, error) {
    handler := NewReceiverHandler(f.config, destPath)
    return NewChannel(f.config, handler), nil
}
```

#### 3.2 Update DataChannelService
**Files to modify:**
- `internal/transport/data_channel_service.go`

**Backward compatibility strategy:**
```go
type DataChannelService struct {
    config  *config.Config
    factory *ChannelFactory
    channel *Channel
    
    // Legacy fields (deprecated)
    senderChannel   *SenderChannel   // Keep for gradual migration
    receiverChannel *ReceiverChannel // Keep for gradual migration
}

// Existing methods maintain same signatures
func (s *DataChannelService) CreateFileSenderDataChannel(ctx context.Context, peerConn *webrtc.PeerConnection, label string, filePath string) error {
    // New implementation using generic channel
    var err error
    s.channel, err = s.factory.CreateSenderChannel(filePath)
    if err != nil {
        return err
    }
    return s.channel.CreateDataChannel(ctx, peerConn, label)
}
```

### Phase 4: Protocol Enhancement (Week 4)

#### 4.1 Acknowledgement Protocol
**Files to modify:**
- `internal/transport/sender_handler.go`
- `internal/transport/receiver_handler.go`

**Key enhancements:**
- [ ] Implement full acknowledgement protocol from specification
- [ ] Add timeout and retry mechanisms
- [ ] Implement protocol version negotiation
- [ ] Add graceful fallback to legacy protocol

#### 4.2 Error Handling
**Files to create:**
- `internal/transport/errors.go`

**Key tasks:**
- [ ] Define standard error codes and types
- [ ] Implement error propagation from receiver to sender
- [ ] Add error recovery mechanisms
- [ ] Implement proper cleanup on errors

### Phase 5: Testing and Validation (Week 5)

#### 5.1 Integration Testing
**Files to create:**
- `internal/transport/integration_test.go`  
- `test/end_to_end_test.go`

**Key tests:**
- [ ] Full file transfer with acknowledgements
- [ ] Error scenario testing (disk full, permission denied, etc.)
- [ ] Timeout and retry testing
- [ ] Protocol version compatibility testing
- [ ] Large file transfer testing

#### 5.2 Performance Testing
**Files to create:**
- `test/performance_test.go`
- `test/benchmark_test.go`

**Key metrics:**
- [ ] Transfer throughput comparison (old vs new)
- [ ] Memory usage analysis
- [ ] CPU usage analysis
- [ ] Latency impact of acknowledgements

### Phase 6: Deployment and Cleanup (Week 6)

#### 6.1 Feature Flag Implementation
**Files to modify:**
- `internal/config/config.go`

```go
type WebRTCConfig struct {
    // ... existing fields
    UseGenericChannel    bool `json:"use_generic_channel"`
    EnableAcknowledgements bool `json:"enable_acknowledgements"`
    LegacyProtocolMode   bool `json:"legacy_protocol_mode"`
}
```

#### 6.2 Gradual Migration
**Migration strategy:**
1. Deploy with feature flag disabled (old behavior)
2. Enable generic channel, but with legacy protocol mode
3. Enable acknowledgements for new transfers
4. Monitor and validate in production
5. Remove legacy code after validation period

#### 6.3 Legacy Code Removal
**Files to deprecate:**
- `internal/transport/sender_channel.go`
- `internal/transport/receiver_channel.go`

**Deprecation timeline:**
- Week 6: Mark as deprecated with comments
- Week 8: Remove from new deployments
- Week 12: Full removal after validation

## File Structure Changes

### New Structure
```
internal/transport/
├── channel.go              # Generic Channel implementation
├── handler.go              # MessageHandler interface and BaseHandler
├── message.go              # Message types and protocol definitions
├── sender_handler.go       # Sender-specific message handling
├── receiver_handler.go     # Receiver-specific message handling  
├── channel_factory.go      # Factory for creating channels
├── errors.go               # Error types and handling
├── data_channel_service.go # Updated service (backward compatibility)
├── sender_channel.go       # [DEPRECATED] Legacy sender
├── receiver_channel.go     # [DEPRECATED] Legacy receiver
└── legacy_adapter.go       # [TEMPORARY] Compatibility layer
```

### Test Structure
```
internal/transport/
├── message_test.go
├── channel_test.go
├── sender_handler_test.go
├── receiver_handler_test.go
├── integration_test.go
└── handler_test_utils.go
```

## Backward Compatibility Strategy

### API Compatibility
- All existing public methods maintain same signatures
- DataChannelService interface unchanged
- Configuration options remain backward compatible
- Legacy protocol detection and fallback

### Migration Path
1. **Parallel Implementation**: New code alongside old code
2. **Feature Flags**: Control which implementation is used
3. **Gradual Rollout**: Enable new features incrementally
4. **Validation Period**: Extended testing before legacy removal

### Rollback Plan
- Feature flags allow instant rollback
- Legacy code remains available during transition
- Database/config changes are additive only
- Monitoring and alerting for performance regressions

## Testing Strategy

### Unit Tests
- Message serialization/deserialization
- Handler state machine transitions
- Error handling and recovery
- Timeout and retry mechanisms

### Integration Tests  
- End-to-end file transfer scenarios
- Protocol version negotiation
- Error propagation and handling
- Performance benchmarking

### Compatibility Tests
- Legacy protocol compatibility
- Mixed version environments
- Backward compatibility validation

## Risk Mitigation

### Technical Risks
- **WebRTC Compatibility**: Extensive testing with different browsers/versions
- **Performance Impact**: Benchmark acknowledgement overhead
- **State Synchronization**: Careful handling of concurrent state changes
- **Memory Leaks**: Proper cleanup of handlers and channels

### Operational Risks
- **Breaking Changes**: Comprehensive backward compatibility testing
- **Performance Degradation**: Monitoring and rollback procedures
- **Configuration Complexity**: Clear documentation and validation

## Success Metrics

### Functional Metrics
- [ ] 100% backward compatibility maintained
- [ ] All existing tests pass with new implementation
- [ ] Enhanced error detection and reporting
- [ ] Successful acknowledgement protocol implementation

### Performance Metrics
- [ ] Transfer throughput within 5% of current performance
- [ ] Memory usage increase less than 10%
- [ ] Acknowledgement latency under 100ms
- [ ] Error recovery time under 5 seconds

### Quality Metrics
- [ ] Code coverage above 80% for new components
- [ ] Zero critical security vulnerabilities
- [ ] Documentation coverage for all public APIs
- [ ] Performance benchmarks pass

## Timeline Summary

| Phase | Duration | Key Deliverables |
|-------|----------|------------------|
| 1 | Week 1 | Core interfaces and Channel implementation |
| 2 | Week 2 | Sender and Receiver handlers |  
| 3 | Week 3 | Channel factory and DataChannelService integration |
| 4 | Week 4 | Acknowledgement protocol and error handling |
| 5 | Week 5 | Comprehensive testing and validation |
| 6 | Week 6 | Deployment and legacy cleanup |

**Total Timeline**: 6 weeks for full implementation and validation

This implementation plan provides a structured approach to refactoring YAPFS while maintaining stability and backward compatibility throughout the migration process.