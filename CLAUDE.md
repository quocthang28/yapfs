# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

YAPFS (Yet Another P2P File Sharing) is a secure peer-to-peer file sharing utility built with WebRTC data channels. It allows direct file transfers between two machines without requiring a central server. Users manually exchange SDP offers and answers to establish secure peer-to-peer connections for file transfer.

## Architecture

The application follows Go CLI best practices with proper separation of concerns and direct dependency injection:

### Directory Structure
```
yapfs/
├── cmd/                     # CLI commands (Cobra-based)
│   ├── root.go             # Root command and service wiring
│   ├── send.go             # Send command implementation
│   └── receive.go          # Receive command implementation
├── internal/
│   ├── app/                # Application service layer
│   │   ├── sender.go       # Sender application logic
│   │   └── receiver.go     # Receiver application logic
│   ├── config/
│   │   └── config.go       # Configuration management
│   ├── file/               # File handling service layer
│   │   └── service.go      # File operations, readers, writers
│   ├── webrtc/             # WebRTC service layer
│   │   ├── peer.go         # Peer connection service
│   │   ├── data_channel.go # Data channel + flow control
│   │   └── signaling.go    # SDP exchange service
│   └── ui/                 # User interface layer
│       └── interactive.go  # Console-based UI implementation
└── main.go                 # Minimal entry point
```

### Core Services

#### **PeerService** (`internal/webrtc/peer.go`)
Manages WebRTC peer connection lifecycle:
- Creates peer connections with ICE server configuration
- Sets up connection state change handlers
- Handles connection lifecycle (create, monitor, close)
- Provides default state handling for failed/closed connections

#### **DataChannelService** (`internal/webrtc/data_channel.go`)
Handles data channels, flow control, and file transfer logic:
- Creates sender data channels with flow control
- Sets up receiver data channel handlers
- Implements file transfer with progress monitoring
- Manages buffering and throughput reporting
- Handles EOF signaling for transfer completion

#### **SignalingService** (`internal/webrtc/signaling.go`)
Manages SDP offer/answer exchange and encoding/decoding:
- Creates and sets SDP offers and answers
- Handles remote description setting
- Waits for ICE candidate gathering completion
- Encodes/decodes session descriptions to/from base64

#### **ConsoleUI** (`internal/ui/interactive.go`)
Handles user input/output for SDP exchange:
- Displays SDP for manual exchange between peers
- Prompts user to input SDP from remote peer
- Shows instructional messages and progress updates
- Provides console-based interactive experience

#### **FileService** (`internal/file/service.go`)
Manages file operations for P2P file sharing:
- Opens files for reading with metadata (FileReader)
- Creates files for writing (FileWriter)
- Provides file information (FileInfo)
- Formats file sizes in human-readable format
- Handles directory creation for destination paths

#### **SenderApp/ReceiverApp** (`internal/app/`)
Coordinate the complete file transfer application flow:
- **SenderApp**: Orchestrates offer creation, file sending, and connection management
- **ReceiverApp**: Handles answer generation, file receiving, and completion signaling
- Both coordinate between WebRTC, UI, and file services

### Design Principles
- **Direct dependency injection**: Concrete types are injected directly without interfaces
- **Separation of concerns**: Clear boundaries between CLI, app logic, WebRTC, file handling, and UI
- **Configuration management**: Centralized config with validation
- **Error handling**: Proper error propagation with context
- **Exported types**: All service types are exported for direct usage

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

## File Transfer Implementation

### How Files are Sent

Files are transferred using a **chunked streaming approach** with the following process:

1. **File Reading**: Files are read sequentially using a fixed-size buffer (default 1 KB chunks)
2. **Chunked Transmission**: Each read operation creates a chunk that is sent immediately via WebRTC data channel
3. **Streaming**: The file is processed in a streaming fashion - chunks are read and sent continuously until EOF
4. **EOF Signaling**: When the entire file has been read, a special "EOF" message is sent to signal completion

### Buffer Management and Flow Control

The implementation includes sophisticated flow control to prevent overwhelming the WebRTC data channel:

#### Configuration Constants
- **PacketSize**: 1 KB (1024 bytes) - size of each chunk read from file
- **BufferedAmountLowThreshold**: 512 KB - threshold for resuming transmission  
- **MaxBufferedAmount**: 1 MB - maximum buffer size before pausing transmission
- **ThroughputReportInterval**: 1 second - frequency of progress updates

#### Flow Control Mechanism
```go
// Read file in 1KB chunks
buffer := make([]byte, d.config.WebRTC.PacketSize)
for {
    n, err := fileReader.Read(buffer)
    if err == io.EOF {
        dataChannel.Send([]byte("EOF"))  // Signal completion
        break
    }
    
    data := buffer[:n]  // Only send actual bytes read
    dataChannel.Send(data)
    
    // Flow control: pause if buffer is too full
    if dataChannel.BufferedAmount() > d.config.WebRTC.MaxBufferedAmount {
        <-sendMoreCh  // Wait for buffer to drain
    }
}
```

#### Key Features
- **Non-blocking**: File reading happens in a separate goroutine
- **Backpressure**: Transmission pauses when WebRTC buffer exceeds 1MB
- **Resume**: Transmission resumes when buffer drops below 512KB threshold
- **Progress Monitoring**: Real-time throughput and percentage completion tracking
- **Memory Efficient**: Only 1KB buffer in memory at any time, regardless of file size

This approach allows efficient transfer of files of any size while maintaining stable memory usage and preventing network congestion.

## Dependencies

This example uses the Pion WebRTC library (`github.com/pion/webrtc/v4`). The module dependencies are managed at the parent directory level in the main pion/webrtc project.