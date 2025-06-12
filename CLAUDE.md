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
│   │   └── file.go         # File operations, readers, writers
│   ├── transport/          # Transport service layer (WebRTC)
│   │   ├── peer.go         # Peer connection service
│   │   ├── data_channel.go # Data channel facade
│   │   ├── signaling.go    # SDP exchange service
│   │   ├── sender.go       # File sending logic
│   │   └── receiver.go     # File receiving logic
│   └── ui/                 # User interface layer
│       ├── interactive.go  # Console-based UI implementation
│       └── progress.go     # Progress reporting and display
└── main.go                 # Minimal entry point
```

### Core Services

#### **PeerService** (`internal/transport/peer.go`)
Manages WebRTC peer connection lifecycle:
- Creates peer connections with ICE server configuration
- Sets up connection state change handlers
- Handles connection lifecycle (create, monitor, close)
- Provides default state handling for failed/closed connections

#### **DataChannelService** (`internal/transport/data_channel.go`)
Facade service that composes sender and receiver channels:
- Delegates sender operations to `SenderChannel`
- Delegates receiver operations to `ReceiverChannel`
- Maintains backward compatibility with existing API
- Provides unified interface for data channel operations

#### **SenderChannel** (`internal/transport/sender.go`)
Handles file sending logic:
- Creates sender data channels with flow control
- Implements chunked file reading and transmission
- Manages flow control and buffering
- Handles EOF signaling for transfer completion

#### **ReceiverChannel** (`internal/transport/receiver.go`)
Handles file receiving logic:
- Sets up receiver data channel handlers
- Manages incoming data channel messages through event-driven channels
- Provides channel-based communication interface for received data
- Processes EOF signals and error propagation through channels

#### **FileTransferCoordinator** (`internal/transport/coordinator.go`)
Mediates between transport and processing layers using channel-based communication:
- **Eliminates tight coupling** between DataChannelService and DataProcessor
- **Event-driven coordination** using well-defined communication channels
- **Centralized flow control** management across transport and processing layers
- **Progress monitoring** and error handling aggregation
- **SenderChannels**: Coordinates file sending with data requests, responses, flow control, progress, and error channels
- **ReceiverChannels**: Coordinates file receiving with data delivery, flow control, progress, and error channels
- **CoordinateSender()**: Orchestrates complete sender flow from file preparation to transfer completion
- **CoordinateReceiver()**: Orchestrates complete receiver flow from setup to file writing completion
- **Context-based lifecycle management** with proper cancellation support

#### **SignalingService** (`internal/transport/signaling.go`)
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

#### **ProgressReporter** (`internal/ui/progress.go`)
Provides standardized progress reporting interface for file transfers:
- Defines common interface for progress reporting across different UI implementations
- Handles progress updates with throughput, bytes transferred, and completion percentage
- Reports errors and completion status with consistent formatting
- Supports console-based progress reporting with emoji indicators

#### **FileService** (`internal/file/file.go`)
Manages file operations for P2P file sharing:
- Opens files for reading with metadata (FileReader)
- Creates files for writing (FileWriter)
- Provides file information (FileInfo)
- Formats file sizes in human-readable format
- Handles directory creation for destination paths

#### **SenderApp/ReceiverApp** (`internal/app/`)
Coordinate the complete file transfer application flow using FileTransferCoordinator and a flexible options-based approach:
- **SenderApp**: Orchestrates offer creation, file sending, and connection management via `Run(*SenderOptions)` using FileTransferCoordinator
- **ReceiverApp**: Handles answer generation, file receiving, and completion signaling via `Run(*ReceiverOptions)` using FileTransferCoordinator
- **SenderOptions**: Configuration struct with required `FilePath` field for extensibility
- **ReceiverOptions**: Configuration struct with required `DstPath` field for extensibility
- Both use FileTransferCoordinator for decoupled communication between transport, processing, and UI layers
- Context-based coordination with proper cancellation and timeout support

### Service Interactions and Dependencies

The application uses **direct dependency injection** with concrete service types and **channel-based coordination** through FileTransferCoordinator for decoupled service communication.

#### Service Dependency Graph
```
SenderApp/ReceiverApp (orchestrators)
├── PeerService (manages WebRTC connections)
├── SignalingService (handles SDP exchange)  
├── FileTransferCoordinator (mediates transport ↔ processing communication)
│   ├── DataChannelService (manages WebRTC data channels)
│   └── DataProcessor (handles file I/O operations)
├── ConsoleUI (user interaction)
└── Config (application configuration)

Channel-based Communication:
FileTransferCoordinator ↔ DataChannelService (via SenderChannels/ReceiverChannels)
FileTransferCoordinator ↔ DataProcessor (via file operation methods)
```

#### Service Instantiation (in `cmd/root.go`)
Services are created and injected using concrete constructors with FileTransferCoordinator managing cross-layer communication:

```go
func createServices() (*transport.PeerService, *transport.DataChannelService, *transport.SignalingService, *ui.ConsoleUI, *processor.DataProcessor) {
    stateHandler := &transport.DefaultConnectionStateHandler{}
    signalingService := transport.NewSignalingService()
    peerService := transport.NewPeerService(cfg, stateHandler)
    consoleUI := ui.NewConsoleUI()
    dataChannelService := transport.NewDataChannelService(cfg)
    dataProcessor := processor.NewDataProcessor()
    
    // FileTransferCoordinator is created within SenderApp/ReceiverApp constructors
    // coordinator := transport.NewFileTransferCoordinator(cfg, dataProcessor, dataChannelService)
    
    return peerService, dataChannelService, signalingService, consoleUI, dataProcessor
}
```

#### Send Operation Service Flow

1. **Connection Setup**
   ```go
   // PeerService creates and monitors WebRTC connection
   pc, err := s.peerService.CreatePeerConnection()
   s.peerService.SetupConnectionStateHandler(pc, "sender")
   ```

2. **Coordinator-Based File Transfer Setup** 
   ```go
   // FileTransferCoordinator orchestrates all file transfer operations
   ctx := context.Background()
   doneCh, err := s.coordinator.CoordinateSender(ctx, pc, opts.FilePath)
   
   // Coordinator internally:
   // - Prepares file using DataProcessor
   // - Creates data channel via DataChannelService  
   // - Sets up channel-based communication between transport and processing
   // - Manages flow control, progress monitoring, and error handling
   ```

3. **SDP Exchange**
   ```go
   // SignalingService creates offer and waits for ICE gathering
   _, err = s.signalingService.CreateOffer(pc)
   err = s.signalingService.WaitForICEGathering(pc)
   encodedOffer, err := s.signalingService.EncodeSessionDescription(*finalOffer)
   
   // ConsoleUI handles user interaction
   err = s.ui.OutputSDP(encodedOffer, "Offer")
   answer, err := s.ui.InputSDP("Answer")
   
   // SignalingService processes answer
   answerSD, err := s.signalingService.DecodeSessionDescription(answer)
   err = s.signalingService.SetRemoteDescription(pc, answerSD)
   ```

4. **Channel-Based File Transfer** (coordinated by FileTransferCoordinator)
   - **SenderChannels**: Data requests, responses, flow control, progress, errors
   - **Transport Layer**: Manages WebRTC data channel and buffer state
   - **Processing Layer**: Handles file reading and chunking
   - **Coordinator**: Mediates communication, aggregates progress, manages flow control

#### Receive Operation Service Flow

1. **Setup and SDP Exchange**
   ```go
   // PeerService creates connection
   pc, err := r.peerService.CreatePeerConnection()
   
   // FileTransferCoordinator orchestrates receiver setup
   ctx := context.Background()
   doneCh, err := r.coordinator.CoordinateReceiver(ctx, pc, opts.DstPath)
   
   // Coordinator internally:
   // - Sets up data channel handlers via DataChannelService
   // - Prepares file receiving via DataProcessor  
   // - Creates ReceiverChannels for communication
   // - Starts monitoring goroutines for progress and errors
   
   // ConsoleUI and SignalingService handle SDP exchange
   offer, err := r.ui.InputSDP("Offer")
   // ... SignalingService processes offer and creates answer
   ```

2. **Channel-Based File Reception** (coordinated by FileTransferCoordinator)
   - **ReceiverChannels**: Data delivery, flow control, progress, errors
   - **Transport Layer**: Receives WebRTC data channel messages, forwards to channels
   - **Processing Layer**: Writes received data chunks to file
   - **Coordinator**: Manages file lifecycle, aggregates progress, handles completion

#### Key Communication Patterns

**Channel-Based Coordination:**
```go
// FileTransferCoordinator uses typed channels for cross-layer communication
type SenderChannels struct {
    DataRequest  chan struct{}                 // Transport → Processing: Request next chunk
    DataResponse chan processor.DataChunk      // Processing → Transport: Provide chunk data  
    FlowControl  chan FlowControlEvent         // Bidirectional flow control signaling
    Progress     chan ProgressUpdate           // Progress reporting aggregation
    Error        chan error                    // Error propagation across layers
    Complete     chan struct{}                 // Transfer completion signaling
}

// Progress flows: DataProcessor → Coordinator → Application Layer
// Data flows: DataProcessor → Coordinator → DataChannelService → WebRTC
```

**Flow Control Coordination:**
```go
// Centralized flow control managed by FileTransferCoordinator
// Transport layer signals buffer state:
s.dataChannel.OnBufferedAmountLow(func() {
    channels.FlowControl <- FlowControlEvent{Type: "resume"}
})

// Coordinator mediates between transport buffer state and processing rate
if s.dataChannel.BufferedAmount() > s.config.WebRTC.MaxBufferedAmount {
    return fmt.Errorf("buffer full, flow control required")
}
```

**Error Propagation:**
- Errors propagate through dedicated error channels to FileTransferCoordinator
- Coordinator aggregates and forwards errors to Application layer
- Context-based cancellation ensures proper cleanup across all services
- UI displays coordinated error messages with consistent formatting

### Design Principles
- **Direct dependency injection**: Concrete types are injected directly without interfaces
- **Channel-based decoupling**: FileTransferCoordinator eliminates tight coupling between transport and processing layers
- **Event-driven architecture**: Services communicate through well-defined channels for asynchronous coordination  
- **Separation of concerns**: Clear boundaries between CLI, app logic, transport, data processing, and UI with coordinator mediation
- **Centralized coordination**: FileTransferCoordinator manages cross-layer communication, flow control, and progress aggregation
- **Context-based lifecycle**: Proper cancellation and timeout support throughout the application
- **Options-based API**: Flexible configuration using struct-based options pattern
- **Configuration management**: Centralized config with validation, environment variable support via Viper
- **Flag handling**: Struct-based flag definitions with built-in Cobra validation
- **Error handling**: Channel-based error propagation with centralized aggregation and context support
- **Exported types**: All service types are exported for direct usage with coordinator managing interactions

### FileTransferCoordinator Architecture Details

#### Channel Communication Patterns

The FileTransferCoordinator implements a sophisticated channel-based communication system that eliminates tight coupling between the transport and processing layers:

**Sender Communication Flow:**
```go
// 1. Coordinator creates SenderChannels for communication
channels := &SenderChannels{
    DataRequest:  make(chan struct{}, 1),           // Transport requests chunks
    DataResponse: make(chan processor.DataChunk, 10), // Processing provides chunks
    FlowControl:  make(chan FlowControlEvent, 1),    // Flow control signaling
    Progress:     make(chan ProgressUpdate, 1),      // Progress aggregation
    Error:        make(chan error, 1),               // Error propagation
    Complete:     make(chan struct{}),               // Completion signaling
}

// 2. Transport layer (SenderChannel) communicates buffer state
s.dataChannel.OnBufferedAmountLow(func() {
    channels.FlowControl <- FlowControlEvent{Type: "resume"}
})

// 3. Processing layer (DataProcessor) provides data chunks
dataCh, errCh := dataProcessor.StartFileTransfer(chunkSize)

// 4. Coordinator mediates all communication between layers
```

**Receiver Communication Flow:**
```go
// 1. Coordinator creates ReceiverChannels for communication  
channels := &ReceiverChannels{
    DataReceived: make(chan processor.DataChunk, 10), // Incoming data delivery
    FlowControl:  make(chan FlowControlEvent, 1),     // Flow control events
    Progress:     make(chan ProgressUpdate, 1),       // Progress reporting
    Error:        make(chan error, 1),                // Error propagation  
    Complete:     chan struct{}),                     // Completion signaling
}

// 2. Transport layer forwards received data through channels
dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
    channels.DataReceived <- processor.DataChunk{Data: msg.Data, EOF: false}
})

// 3. Coordinator handles file writing and progress aggregation
```

#### Benefits of Channel-Based Architecture

1. **Eliminates Direct Dependencies**: Transport layer no longer directly depends on DataProcessor
2. **Enables Independent Testing**: Each service can be tested in isolation with mock channels
3. **Centralized Flow Control**: All flow control logic managed in one place by coordinator
4. **Consistent Error Handling**: Errors propagate through dedicated channels to coordinator
5. **Progress Aggregation**: Progress updates from multiple sources consolidated by coordinator
6. **Context-Based Cancellation**: Proper cleanup across all services using context cancellation
7. **Flexible Communication**: Easy to add new communication patterns without changing service interfaces

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

Optional configuration file support:
```bash
./yapfs send --file /path/to/file --config /path/to/config.yaml
./yapfs receive --dst /path/to/file --config ~/.yapfs.yaml
```

Environment variable support:
```bash
YAPFS_SEND_FILE=/path/to/file ./yapfs send
YAPFS_RECEIVE_DST=/path/to/save ./yapfs receive
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

## Code Organization

### Application Layer Pattern
The application follows a clean layered architecture:

1. **CLI Layer** (`cmd/`): Flag parsing, validation, and user interface
2. **Application Layer** (`internal/app/`): Business logic orchestration with options pattern
3. **Service Layer** (`internal/transport/`, `internal/processor/`, `internal/ui/`): Domain-specific services
4. **Configuration Layer** (`internal/config/`): Centralized configuration management

### Extensibility Guidelines
When adding new features:

1. **New Flags**: Add to flag structs in `cmd/` files with comments for future expansion
2. **New Options**: Add to options structs in `internal/app/` files
3. **Validation**: Use Cobra's `PreRunE` for flag validation
4. **Services**: Create new services in appropriate `internal/` subdirectories
5. **Configuration**: Add new config fields to `internal/config/config.go`

This pattern ensures maintainable, testable code with clear separation of concerns.

## CLI Architecture

### Flag Handling
The application uses a modern, flexible flag handling approach:

#### Command Structure
```go
type SendFlags struct {
    FilePath string // Required: path to file to send
    // Future flags can be easily added here:
    // Verbose  bool
    // Timeout  int
}

type ReceiveFlags struct {
    DstPath string // Required: destination path to save file
    // Future flags can be easily added here:
    // Verbose  bool
    // Timeout  int
}
```

#### Application Options
```go
type SenderOptions struct {
    FilePath string // Required: path to file to send
}

type ReceiverOptions struct {
    DstPath string // Required: destination path to save file
}
```

#### Unified Run Methods
- **SenderApp.Run(ctx, *SenderOptions)**: Single method handles all sender functionality
- **ReceiverApp.Run(ctx, *ReceiverOptions)**: Single method handles all receiver functionality
- No duplicate logic between different run methods
- Extensible design for future feature additions

### Configuration Support
- **Viper integration**: Environment variable support with `YAPFS_` prefix
- **Config files**: YAML configuration files (default: `~/.yapfs.yaml`)
- **Cobra validation**: Built-in flag validation using `PreRunE`
- **Extensible**: Easy to add new flags without code duplication

## Dependencies

Core dependencies:
- **Pion WebRTC**: `github.com/pion/webrtc/v4` - WebRTC implementation
- **Cobra**: `github.com/spf13/cobra` - CLI framework
- **Viper**: `github.com/spf13/viper` - Configuration management

## Future Improvements

A comprehensive roadmap of planned enhancements is documented in `FUTURE_IMPROVEMENTS.md`, including:

### Planned Features
1. **File Data Integrity**: SHA-256 checksums, chunk verification, resume capability for interrupted transfers
2. **Enhanced Security**: Peer authentication, application-layer encryption, path validation, directory traversal protection
3. **Automated SDP Exchange**: Cloudflare Worker + KV integration for seamless session sharing without manual copy/paste

### Implementation Priority
- **Short term**: File integrity verification, basic security hardening, Cloudflare Worker SDP exchange
- **Medium term**: Resume capability, peer authentication mechanisms  
- **Long term**: Advanced encryption, rate limiting, web UI

Reference `FUTURE_IMPROVEMENTS.md` for detailed implementation specifications, security considerations, and architectural guidance for these enhancements.