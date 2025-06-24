# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

YAPFS (Yet Another P2P File Sharing) is a WebRTC-based peer-to-peer file transfer application written in Go. It enables direct file transfers between machines without intermediate servers, using Firebase Realtime Database for SDP signaling exchange.

## Build and Development Commands

```bash
# Build the application
go build -o yapfs

# Build with specific output location  
go build -o bin/yapfs

# Run without building
go run main.go send --file /path/to/file
go run main.go receive --dst /path/to/destination

# Check dependencies
go mod tidy
go mod download
```

## Application Usage

```bash
# Send a file
./yapfs send --file /path/to/your/file

# Receive a file
./yapfs receive --dst /path/to/save/file


## Architecture Overview

### Package Structure
- **`cmd/`** - CLI command definitions (Cobra-based)
- **`internal/app/`** - Application orchestrators (sender.go, receiver.go)
- **`internal/transport/`** - WebRTC transport layer with channel/handler architecture
- **`internal/signalling/`** - SDP exchange via Firebase Realtime Database
- **`internal/processor/`** - File I/O and data processing services
- **`internal/config/`** - Configuration management and validation
- **`internal/reporter/`** - Progress reporting and UI components
- **`pkg/utils/`** - Utility functions for codes, files, and SDP handling
- **`pkg/types/`** - Type definitions and data structures

### Key Data Flow
1. **Sender**: Creates WebRTC offer → uploads to Firebase → waits for answer → transfers file
2. **Receiver**: Retrieves offer from Firebase → creates answer → uploads answer → receives file
3. **Transfer**: Direct P2P via WebRTC data channels with flow control and progress monitoring

### Configuration
- Uses `config.json` in root directory (see `example-config.json`)
- Configuration layered: defaults → file → environment variables
- Supports Firebase settings for automatic SDP exchange
- WebRTC settings: ICE servers, chunk size, buffer thresholds

### Service Architecture
Services follow composition and dependency injection patterns:
- **PeerService** - WebRTC peer connection lifecycle with enhanced state management
- **Channel** - Generic bidirectional WebRTC data channel with configurable handlers
- **MessageHandler Interface** - Contract for handling messages and channel lifecycle events
- **SenderHandler/ReceiverHandler** - Role-specific message handling with state machines
- **ChannelFactory** - Factory functions for creating specialized sender/receiver channels
- **SignalingService** - Orchestrates SDP offer/answer exchange
- **DataProcessor** - File operations with checksum verification
- **ProgressReporter** - Dedicated progress tracking and UI updates

## Important Implementation Details

### WebRTC Integration
- Uses `github.com/pion/webrtc/v4` for WebRTC functionality
- Data channels configured for ordered delivery
- Flow control via `max_buffered_amount` and `buffered_amount_low_threshold`
- ICE servers configurable for NAT traversal

### File Transfer Protocol
Uses a structured message-based protocol with explicit state management:

**Message Types:**
- Control: `MSG_READY`, `MSG_METADATA_ACK`, `MSG_TRANSFER_COMPLETE`, `MSG_ERROR`
- Data: `MSG_METADATA`, `MSG_FILE_DATA`, `MSG_EOF`

**Transfer Flow:**
1. State machine initialization (sender/receiver)
2. Metadata transmission (JSON with file info, size, checksum)
3. Chunked file data transfer (configurable chunk size)
4. SHA-256 checksum verification for integrity
5. Progress reporting with throughput calculations
6. Completion acknowledgment through structured messaging

### State Machine Architecture
Both sender and receiver implement explicit state machines for reliable transfer coordination:

**Sender States:**
- `SenderInitializing` → `SenderWaitingForReady` → `SenderSendingMetadata` → `SenderWaitingForMetadataAck` → `SenderTransferringData` → `SenderWaitingForCompletion` → `SenderCompleted`

**Receiver States:**
- `ReceiverInitializing` → `ReceiverReady` → `ReceiverReceivingMetadata` → `ReceiverPreparingFile` → `ReceiverReceivingData` → `ReceiverCompleted`

**Handler Pattern:**
- `BaseHandler` provides common functionality (context management, cancellation)
- Role-specific handlers extend BaseHandler with sender/receiver logic
- Message validation based on current state prevents protocol violations

### Error Handling and Cleanup
- Context-based cancellation throughout the application
- Graceful shutdown with proper resource cleanup
- Connection state callbacks handle WebRTC lifecycle events
- Partial file cleanup on transfer failures

### Concurrency Patterns
- Handler-based message processing with separate incoming/outgoing loops
- Channel-based communication for progress updates and completion callbacks
- Goroutine coordination with sync.Once patterns for graceful shutdown
- Context cancellation for coordinated shutdowns
- Thread-safe state management with read-write mutexes
- Non-blocking progress reporting with buffered channels

## Configuration Files

- **`config.json`** - Main configuration (use `example-config.json` as template)
- **Firebase service account JSON** - For Firebase Realtime Database access

## Dependencies

Key external dependencies:
- `github.com/pion/webrtc/v4` - WebRTC implementation
- `firebase.google.com/go/v4` - Firebase SDK
- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration management
- `github.com/schollz/progressbar/v3` - Progress display