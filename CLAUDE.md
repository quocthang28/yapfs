# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

YAPFS (Yet Another P2P File Sharing) is a secure peer-to-peer file sharing utility built with WebRTC data channels. It allows direct file transfers between two machines without requiring a central server. Users manually exchange SDP offers and answers to establish secure peer-to-peer connections for file transfer.

## New Architecture (Refactored)

The application now follows Go CLI best practices with proper separation of concerns:

### Directory Structure
```
yapfs/
├── cmd/                     # CLI commands (Cobra-based)
│   ├── root.go             # Root command and service wiring
│   ├── send.go             # Send command implementation
│   └── receive.go          # Receive command implementation
├── internal/
│   ├── app/                # Application service layer
│   │   ├── interfaces.go   # App service interfaces
│   │   ├── sender.go       # Sender application logic
│   │   └── receiver.go     # Receiver application logic
│   ├── config/
│   │   └── config.go       # Configuration management
│   ├── webrtc/             # WebRTC service layer
│   │   ├── interfaces.go   # WebRTC service interfaces
│   │   ├── peer.go         # Peer connection service
│   │   ├── datachannel.go  # Data channel + flow control
│   │   └── signaling.go    # SDP exchange service
│   └── ui/                 # User interface layer
│       ├── interfaces.go   # UI interfaces
│       └── interactive.go  # Console-based UI implementation
└── main.go                 # Minimal entry point
```

### Key Services
- **PeerService**: Manages WebRTC peer connection lifecycle
- **DataChannelService**: Handles data channels, flow control, and file transfer logic
- **SignalingService**: Manages SDP offer/answer exchange and encoding
- **InteractiveUI**: Handles user input/output for SDP exchange
- **SenderApp/ReceiverApp**: Coordinate file transfer application flow

### Design Principles
- **Interface-driven**: All services implement interfaces for testability
- **Dependency injection**: Services are wired together in cmd/root.go
- **Separation of concerns**: Clear boundaries between CLI, app logic, WebRTC, and UI
- **Configuration management**: Centralized config with validation
- **Error handling**: Proper error propagation with context

## Running the Code

Build the application:
```bash
go build -o yapfs
```

Send a file:
```bash
./yapfs send --file /path/to/your/file
```

Receive a file:
```bash
./yapfs receive --dst /path/to/save/received/file
```

The program will:
1. Display instructions for manual SDP exchange
2. Generate/accept SDP offers and answers through user input
3. Establish WebRTC connection between separate instances
4. Start file transfer with progress monitoring and throughput measurements
5. Complete transfer and exit when file is fully sent/received

## Usage Flow

1. Start sender: `./yapfs send --file /path/to/file`
2. Start receiver: `./yapfs receive --dst /path/to/save/file`
3. Copy the SDP offer from sender and paste into receiver
4. Copy the answer SDP from receiver back to sender
5. Watch the file transfer progress and throughput statistics
6. File transfer completes automatically when done

## Buffer Management Constants

- `bufferedAmountLowThreshold`: 512 KB - threshold for resuming transmission
- `maxBufferedAmount`: 1 MB - maximum buffer size before pausing transmission

## Dependencies

This example uses the Pion WebRTC library (`github.com/pion/webrtc/v4`). The module dependencies are managed at the parent directory level in the main pion/webrtc project.