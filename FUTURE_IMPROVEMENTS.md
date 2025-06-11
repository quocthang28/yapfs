# Future Improvements

This document tracks potential enhancements and improvements for YAPFS (Yet Another P2P File Sharing).

## 1. File Data Integrity

### Current State
- No integrity verification during file transfer
- No mechanism to detect data corruption or transmission errors
- No resume capability for interrupted transfers

### Proposed Improvements

#### Checksums and Verification
- **File-level checksums**: Calculate and verify SHA-256 hash of entire file
- **Chunk-level verification**: Implement per-chunk checksums (CRC32 or xxHash)
- **Progressive verification**: Verify chunks as they arrive rather than after complete transfer
- **Automatic retry mechanism**: Automatically request retransmission of failed/corrupted chunks

#### Implementation Options
```go
type FileMetadata struct {
    Name     string
    Size     int64
    Checksum string // SHA-256 of entire file
}

type Chunk struct {
    Index    int
    Data     []byte
    Checksum uint32 // CRC32 of chunk data
}

type ChunkRequest struct {
    Index int // Request retransmission of specific chunk
}

type ChunkAck struct {
    Index  int  // Chunk index being acknowledged
    Status bool // true = valid, false = corrupted/failed
}
```

#### Resume Capability
- **Partial transfer tracking**: Track received chunks to enable resume
- **Range requests**: Allow requesting specific byte ranges for incomplete files
- **State persistence**: Save transfer state to disk for crash recovery

#### Automatic Error Recovery
- **Chunk validation**: Verify each chunk's CRC32 on receipt
- **Failed chunk detection**: Identify corrupted or missing chunks automatically
- **Selective retransmission**: Request only failed chunks, not entire file
- **Exponential backoff**: Implement retry delays to avoid overwhelming connection
- **Maximum retry limit**: Prevent infinite retry loops for persistently failing chunks

#### Protocol Flow Example
```go
// Receiver validates chunk and sends acknowledgment
func (r *Receiver) ProcessChunk(chunk *Chunk) {
    if validateChecksum(chunk) {
        r.sendAck(ChunkAck{Index: chunk.Index, Status: true})
        r.saveChunk(chunk)
    } else {
        r.sendAck(ChunkAck{Index: chunk.Index, Status: false})
        r.requestRetransmission(ChunkRequest{Index: chunk.Index})
    }
}

// Sender handles acknowledgments and retransmission requests
func (s *Sender) HandleAck(ack *ChunkAck) {
    if !ack.Status {
        s.retryQueue.Add(ack.Index)
    }
    s.markChunkComplete(ack.Index)
}
```

### Benefits
- **Guaranteed file integrity**: Automatic detection and correction of corrupted data
- **Resilient transfers**: Ability to recover from connection failures and network issues
- **Efficient error handling**: Only retransmit failed chunks, not entire files
- **User confidence**: Transfer reliability across unreliable networks
- **Reduced bandwidth waste**: Avoid re-sending successfully received data

## 2. Security Concerns

### Current Vulnerabilities
- No authentication between peers
- No encryption beyond WebRTC's built-in DTLS
- SDP exchange happens in plaintext via manual copy/paste
- No protection against malicious file names or paths
- Potential directory traversal attacks via file paths

### Proposed Security Enhancements

#### Peer Authentication
- **Pre-shared keys**: Simple symmetric key authentication
- **Certificate-based auth**: X.509 certificates for peer verification
- **Password-based pairing**: Short-lived pairing codes

#### Enhanced Encryption
- **Application-layer encryption**: Additional AES-256 encryption on top of DTLS
- **Key derivation**: Proper key derivation from authentication material
- **Forward secrecy**: Ephemeral keys that don't persist after session

#### Path Security
- **Path validation**: Strict validation of destination paths
- **Sandboxing**: Restrict file operations to designated directories
- **Filename sanitization**: Remove dangerous characters and path traversal attempts

#### Implementation Example
```go
type SecureFileTransfer struct {
    AuthKey    []byte // Pre-shared authentication key
    SessionKey []byte // Ephemeral session encryption key
    AllowedDir string // Restricted destination directory
}

func (s *SecureFileTransfer) ValidatePath(path string) error {
    // Validate path is within allowed directory
    // Remove path traversal attempts (../, ..\, etc.)
    // Sanitize filename
}
```

### Additional Security Measures
- **Rate limiting**: Prevent abuse via excessive connection attempts
- **Transfer quotas**: Limit file sizes and transfer counts
- **Audit logging**: Log all transfer attempts and outcomes

## 3. Enhanced UI with Progress Bar and Transfer Statistics

### Current State
- Basic console output with simple throughput reporting
- No visual progress indication during file transfers
- Limited transfer statistics and ETA information
- Progress updates interrupt console flow

### Proposed UI Improvements

#### Real-time Progress Bar
- **Visual progress indicator**: ASCII progress bar showing transfer completion percentage
- **Live statistics**: Real-time display of transfer speed, bytes transferred, and remaining time
- **Non-intrusive updates**: Progress bar updates in-place without scrolling console
- **Completion summary**: Final transfer statistics upon completion

#### Implementation Design
```go
type ProgressInfo struct {
    BytesTransferred int64
    TotalBytes       int64
    ThroughputMbps   float64
    Percentage       float64
    ElapsedTime      time.Duration
    ETADuration      time.Duration
}

type ProgressReporter interface {
    StartProgressDisplay(totalBytes int64)
    UpdateProgress(info ProgressInfo)
    StopProgressDisplay()
    ShowTransferComplete(bytesTransferred int64, elapsedTime time.Duration)
}
```

#### Progress Bar Features
- **Visual bar**: `[===========>           ] 45.2% 2.1MB/5.0MB 1.5MB/s 1m23s ETA: 2m10s`
- **Dynamic width**: Adjusts to terminal width for optimal display
- **Smart formatting**: Human-readable file sizes (KB, MB, GB) and time durations
- **Error handling**: Graceful fallback for non-terminal environments

#### Enhanced Data Channel Integration
```go
// Enhanced throughput reporter with progress callbacks
type ProgressAwareReporter struct {
    ui              ProgressReporter
    totalBytes      int64
    startTime       time.Time
    lastUpdateTime  time.Time
}

func (p *ProgressAwareReporter) OnThroughputUpdate(mbps float64, bytesTransferred int64) {
    elapsed := time.Since(p.startTime)
    percentage := float64(bytesTransferred) / float64(p.totalBytes) * 100
    
    // Calculate ETA based on current throughput
    remainingBytes := p.totalBytes - bytesTransferred
    eta := time.Duration(float64(remainingBytes) / (mbps * 1024 * 1024 / 8)) * time.Second
    
    progress := ProgressInfo{
        BytesTransferred: bytesTransferred,
        TotalBytes:       p.totalBytes,
        ThroughputMbps:   mbps,
        Percentage:       percentage,
        ElapsedTime:      elapsed,
        ETADuration:      eta,
    }
    
    p.ui.UpdateProgress(progress)
}
```

#### Console Management
- **Line clearing**: Proper ANSI escape sequences to clear and update progress line
- **Message handling**: Temporary progress bar clearing for status messages
- **Terminal detection**: Fallback to simple text output for non-interactive terminals

#### Benefits
- **Better user experience**: Clear visual feedback during transfers
- **Informed waiting**: Users can see progress and estimated completion time
- **Professional appearance**: Modern CLI tool appearance with real-time updates
- **Transfer insights**: Detailed statistics help users understand performance

#### Implementation Example
```bash
# Sender with progress bar
$ ./yapfs send --file large_video.mp4
Starting file transfer (Total: 1.2 GB)...
[=============>      ] 67.3% 823MB/1.2GB 15.2MB/s 54s ETA: 26s

# Receiver with progress bar  
$ ./yapfs receive --dst ./downloads/
Ready to receive file...
[==========>         ] 52.1% 425MB/816MB 12.8MB/s 33s ETA: 31s

✓ Transfer complete!
  Total: 1.2 GB
  Time: 1m21s  
  Avg Speed: 15.8 MB/s
```

## 4. SDP Exchange via Cloudflare Worker + KV

### Current Limitations
- Manual copy/paste of SDP is error-prone and cumbersome
- No automation possible for SDP exchange
- Difficult to use programmatically
- Poor user experience for non-technical users

### Proposed Solution: Cloudflare Worker + KV

#### Architecture Overview
```
Sender                    Cloudflare Worker + KV              Receiver
  |                              |                               |
  |---> POST /offer/{id} ------->|                               |
  |     (SDP offer)              |---> Store in KV               |
  |                              |                               |
  |                              |<--- GET /offer/{id} <---------|
  |                              |     (retrieve SDP offer)      |
  |                              |                               |
  |<--- GET /answer/{id} <-------|<--- POST /answer/{id} <-------|
  |     (retrieve SDP answer)    |     (SDP answer)              |
```

#### Cloudflare Worker Implementation
```javascript
// Simplified Cloudflare Worker code
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const [, type, id] = url.pathname.split('/');
    
    if (request.method === 'POST') {
      // Store SDP offer/answer
      const sdp = await request.text();
      await env.SDP_KV.put(`${type}:${id}`, sdp, { expirationTtl: 300 });
      return new Response('OK');
    }
    
    if (request.method === 'GET') {
      // Retrieve SDP offer/answer
      const sdp = await env.SDP_KV.get(`${type}:${id}`);
      return new Response(sdp || 'Not found', { 
        status: sdp ? 200 : 404 
      });
    }
  }
}
```

#### YAPFS Integration
```go
type CloudflareSignaling struct {
    WorkerURL string
    Timeout   time.Duration
}

func (c *CloudflareSignaling) PublishOffer(id string, offer *webrtc.SessionDescription) error {
    // POST SDP offer to Cloudflare Worker
}

func (c *CloudflareSignaling) GetOffer(id string) (*webrtc.SessionDescription, error) {
    // GET SDP offer from Cloudflare Worker
}

func (c *CloudflareSignaling) PublishAnswer(id string, answer *webrtc.SessionDescription) error {
    // POST SDP answer to Cloudflare Worker
}

func (c *CloudflareSignaling) GetAnswer(id string) (*webrtc.SessionDescription, error) {
    // GET SDP answer from Cloudflare Worker
}
```

#### Usage Flow
```bash
# Sender generates and shares a session ID
./yapfs send --file /path/to/file --session abc123

# Receiver uses the same session ID
./yapfs receive --dst /path/to/save --session abc123
```

#### Benefits
- **Automated SDP exchange**: No manual copy/paste required
- **Better UX**: Simple session ID sharing instead of large SDP blobs
- **Programmatic usage**: Easy to integrate into scripts and automation
- **Global availability**: Cloudflare's edge network for low latency
- **Cost-effective**: Cloudflare Workers + KV are very affordable
- **Ephemeral**: SDP data expires quickly (5 minutes) for security

#### Security Considerations
- **Session ID entropy**: Use cryptographically secure random session IDs
- **Rate limiting**: Implement rate limiting in Cloudflare Worker
- **HTTPS only**: Ensure all communication uses HTTPS
- **No sensitive data**: Only store ephemeral SDP data, never file content

#### Implementation Priority
1. **Phase 1**: Basic Cloudflare Worker + KV setup
2. **Phase 2**: YAPFS integration with `--session` flag
3. **Phase 3**: Enhanced security and rate limiting
4. **Phase 4**: Optional: Web UI for even simpler session sharing

## 5. WebRTC Connection Reliability

### Current Limitations
- No retry mechanism for failed ICE gathering
- Single attempt SDP exchange with no fallback
- Connection failures result in immediate termination
- No handling of network connectivity issues during handshake

### Proposed Improvements

#### ICE Exchange Retry Logic
- **Automatic retry**: Retry failed ICE gathering with exponential backoff
- **Connection state monitoring**: Monitor ICE connection state changes
- **Fallback mechanisms**: Try different ICE server configurations on failure
- **Timeout handling**: Configurable timeouts for each phase of connection establishment

#### Implementation Design
```go
type ConnectionRetryConfig struct {
    MaxRetries        int           // Maximum number of retry attempts
    InitialDelay      time.Duration // Initial retry delay
    MaxDelay          time.Duration // Maximum retry delay
    BackoffMultiplier float64       // Exponential backoff multiplier
    ICETimeout        time.Duration // Timeout for ICE gathering
}

type RetryableSignaling struct {
    config     ConnectionRetryConfig
    iceServers []webrtc.ICEServer // Alternative ICE server configurations
}

func (r *RetryableSignaling) CreateOfferWithRetry(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
    var lastErr error
    delay := r.config.InitialDelay
    
    for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
        if attempt > 0 {
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return nil, ctx.Err()
            }
            
            // Increase delay for next attempt
            delay = time.Duration(float64(delay) * r.config.BackoffMultiplier)
            if delay > r.config.MaxDelay {
                delay = r.config.MaxDelay
            }
        }
        
        // Attempt ICE gathering with timeout
        gatherCtx, cancel := context.WithTimeout(ctx, r.config.ICETimeout)
        offer, err := r.attemptOfferCreation(gatherCtx, pc)
        cancel()
        
        if err == nil {
            return offer, nil
        }
        
        lastErr = err
        
        // Try different ICE servers on subsequent attempts
        if attempt < len(r.iceServers) {
            pc.Close()
            pc = r.createPeerConnectionWithICEServers(r.iceServers[attempt])
        }
    }
    
    return nil, fmt.Errorf("failed to create offer after %d attempts: %w", r.config.MaxRetries+1, lastErr)
}

func (r *RetryableSignaling) attemptOfferCreation(ctx context.Context, pc *webrtc.PeerConnection) (*webrtc.SessionDescription, error) {
    offer, err := pc.CreateOffer(nil)
    if err != nil {
        return nil, fmt.Errorf("create offer failed: %w", err)
    }
    
    if err := pc.SetLocalDescription(offer); err != nil {
        return nil, fmt.Errorf("set local description failed: %w", err)
    }
    
    // Wait for ICE gathering with context cancellation
    gatherComplete := webrtc.GatheringCompletePromise(pc)
    select {
    case <-gatherComplete:
        return pc.LocalDescription(), nil
    case <-ctx.Done():
        return nil, fmt.Errorf("ICE gathering timeout: %w", ctx.Err())
    }
}
```

#### Connection State Recovery
```go
type ConnectionMonitor struct {
    pc              *webrtc.PeerConnection
    retryConfig     ConnectionRetryConfig
    onFailure       func() error // Callback to recreate connection
    onSuccess       func()       // Callback when connection recovers
}

func (c *ConnectionMonitor) MonitorConnection(ctx context.Context) {
    c.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
        switch state {
        case webrtc.PeerConnectionStateFailed:
            go c.handleConnectionFailure(ctx)
        case webrtc.PeerConnectionStateConnected:
            if c.onSuccess != nil {
                c.onSuccess()
            }
        case webrtc.PeerConnectionStateDisconnected:
            // Start monitoring for recovery or failure
            go c.monitorReconnection(ctx)
        }
    })
}

func (c *ConnectionMonitor) handleConnectionFailure(ctx context.Context) {
    if c.onFailure != nil {
        if err := c.onFailure(); err != nil {
            // Log failure and potentially terminate
        }
    }
}
```

#### Enhanced Error Handling
- **Specific error types**: Differentiate between network, configuration, and protocol errors
- **User-friendly messages**: Clear error messages with suggested actions
- **Diagnostic information**: Include connection state and timing information in errors
- **Recovery suggestions**: Provide specific steps users can take to resolve issues

#### Configuration Integration
```go
type WebRTCConfig struct {
    // Existing configuration...
    
    // Connection retry configuration
    ConnectionRetry ConnectionRetryConfig `mapstructure:"connection_retry"`
    
    // Alternative ICE servers for failover
    FallbackICEServers [][]webrtc.ICEServer `mapstructure:"fallback_ice_servers"`
}
```

#### Benefits
- **Improved reliability**: Better success rate for WebRTC connections
- **Network resilience**: Handle temporary network issues during connection setup
- **Better diagnostics**: Clear feedback when connections fail permanently
- **Configurable behavior**: Users can adjust retry behavior based on their network conditions
- **Graceful degradation**: Fallback to different ICE server configurations

## 6. Multiple Data Channels for Concurrent File Transfer

### Current Limitations
- Single data channel per WebRTC connection limits transfer throughput
- Files are transferred sequentially in fixed 1KB chunks
- No parallelization of file transfer operations
- Cannot fully utilize available bandwidth for large files

### Proposed Enhancement: Concurrent Data Channels

#### Architecture Overview
```
File Splitting:    [Large File] → [Chunk 1] [Chunk 2] [Chunk 3] [Chunk N]
                        ↓           ↓         ↓         ↓         ↓
Data Channels:    [Channel 1] [Channel 2] [Channel 3] [Channel N]
                        ↓           ↓         ↓         ↓         ↓
Reassembly:       [Chunk 1] [Chunk 2] [Chunk 3] [Chunk N] → [Complete File]
```

#### File Chunking Strategy
```go
type FileChunk struct {
    Index      int    // Chunk sequence number
    Data       []byte // Chunk data
    Size       int64  // Size of this chunk
    IsLast     bool   // Indicates final chunk
    TotalSize  int64  // Total file size
    Checksum   uint32 // CRC32 for integrity
}

type ChunkingConfig struct {
    ChunkSize     int64 // Size per chunk (e.g., 1MB)
    MaxChannels   int   // Maximum concurrent channels (e.g., 4)
    BufferSize    int   // Buffer size per channel
}

func SplitFileIntoChunks(filePath string, config ChunkingConfig) ([]FileChunk, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    stat, err := file.Stat()
    if err != nil {
        return nil, err
    }
    
    totalSize := stat.Size()
    numChunks := int(math.Ceil(float64(totalSize) / float64(config.ChunkSize)))
    chunks := make([]FileChunk, 0, numChunks)
    
    for i := 0; i < numChunks; i++ {
        chunkData := make([]byte, config.ChunkSize)
        n, err := file.Read(chunkData)
        if err != nil && err != io.EOF {
            return nil, err
        }
        
        chunk := FileChunk{
            Index:     i,
            Data:      chunkData[:n],
            Size:      int64(n),
            IsLast:    i == numChunks-1,
            TotalSize: totalSize,
            Checksum:  crc32.ChecksumIEEE(chunkData[:n]),
        }
        
        chunks = append(chunks, chunk)
    }
    
    return chunks, nil
}
```

#### Multi-Channel Data Transfer
```go
type MultiChannelTransfer struct {
    peerConn    *webrtc.PeerConnection
    channels    []*webrtc.DataChannel
    config      ChunkingConfig
    chunks      []FileChunk
    progressCh  chan TransferProgress
    errorCh     chan error
    completeCh  chan bool
}

type TransferProgress struct {
    ChannelID       int
    ChunkIndex      int
    BytesTransferred int64
    TotalBytes      int64
    Throughput      float64
}

func (m *MultiChannelTransfer) StartConcurrentTransfer(ctx context.Context) error {
    // Create multiple data channels
    for i := 0; i < m.config.MaxChannels; i++ {
        channelLabel := fmt.Sprintf("fileTransfer_%d", i)
        channel, err := m.peerConn.CreateDataChannel(channelLabel, nil)
        if err != nil {
            return fmt.Errorf("failed to create data channel %d: %w", i, err)
        }
        
        m.channels = append(m.channels, channel)
        
        // Set up channel event handlers
        m.setupChannelHandlers(i, channel)
    }
    
    // Distribute chunks across channels
    return m.distributeChunksToChannels(ctx)
}

func (m *MultiChannelTransfer) distributeChunksToChannels(ctx context.Context) error {
    // Round-robin distribution of chunks to channels
    chunkQueues := make([][]FileChunk, m.config.MaxChannels)
    
    for i, chunk := range m.chunks {
        channelIndex := i % m.config.MaxChannels
        chunkQueues[channelIndex] = append(chunkQueues[channelIndex], chunk)
    }
    
    // Start concurrent transfers on each channel
    var wg sync.WaitGroup
    for i, queue := range chunkQueues {
        wg.Add(1)
        go func(channelID int, chunks []FileChunk) {
            defer wg.Done()
            m.transferChunksOnChannel(ctx, channelID, chunks)
        }(i, queue)
    }
    
    // Wait for all transfers to complete
    go func() {
        wg.Wait()
        m.completeCh <- true
    }()
    
    return nil
}

func (m *MultiChannelTransfer) transferChunksOnChannel(ctx context.Context, channelID int, chunks []FileChunk) {
    channel := m.channels[channelID]
    
    for _, chunk := range chunks {
        select {
        case <-ctx.Done():
            return
        default:
            // Serialize chunk data
            data, err := json.Marshal(chunk)
            if err != nil {
                m.errorCh <- fmt.Errorf("channel %d: failed to marshal chunk %d: %w", channelID, chunk.Index, err)
                return
            }
            
            // Send chunk with flow control
            err = m.sendWithFlowControl(channel, data)
            if err != nil {
                m.errorCh <- fmt.Errorf("channel %d: failed to send chunk %d: %w", channelID, chunk.Index, err)
                return
            }
            
            // Report progress
            m.progressCh <- TransferProgress{
                ChannelID:        channelID,
                ChunkIndex:       chunk.Index,
                BytesTransferred: chunk.Size,
                TotalBytes:       chunk.TotalSize,
            }
        }
    }
}
```

#### Receiver-Side Reassembly
```go
type ChunkReceiver struct {
    expectedChunks  int
    receivedChunks  map[int]FileChunk // chunk index -> chunk data
    totalSize       int64
    outputFile      *os.File
    progressCh      chan TransferProgress
    mutex           sync.Mutex
}

func (r *ChunkReceiver) HandleIncomingChunk(channelID int, data []byte) error {
    var chunk FileChunk
    if err := json.Unmarshal(data, &chunk); err != nil {
        return fmt.Errorf("failed to unmarshal chunk: %w", err)
    }
    
    // Verify chunk integrity
    if crc32.ChecksumIEEE(chunk.Data) != chunk.Checksum {
        return fmt.Errorf("chunk %d integrity check failed", chunk.Index)
    }
    
    r.mutex.Lock()
    defer r.mutex.Unlock()
    
    // Store received chunk
    r.receivedChunks[chunk.Index] = chunk
    
    // Check if we have all chunks
    if len(r.receivedChunks) == r.expectedChunks {
        return r.reassembleFile()
    }
    
    // Report progress
    r.progressCh <- TransferProgress{
        ChannelID:        channelID,
        ChunkIndex:       chunk.Index,
        BytesTransferred: chunk.Size,
        TotalBytes:       chunk.TotalSize,
    }
    
    return nil
}

func (r *ChunkReceiver) reassembleFile() error {
    // Sort chunks by index
    indices := make([]int, 0, len(r.receivedChunks))
    for index := range r.receivedChunks {
        indices = append(indices, index)
    }
    sort.Ints(indices)
    
    // Write chunks to file in order
    for _, index := range indices {
        chunk := r.receivedChunks[index]
        if _, err := r.outputFile.Write(chunk.Data); err != nil {
            return fmt.Errorf("failed to write chunk %d: %w", index, err)
        }
    }
    
    return r.outputFile.Sync()
}
```

#### Configuration Integration
```go
type MultiChannelConfig struct {
    Enabled       bool  `mapstructure:"enabled"`        // Enable multi-channel transfer
    MaxChannels   int   `mapstructure:"max_channels"`   // Maximum concurrent channels (default: 4)
    ChunkSize     int64 `mapstructure:"chunk_size"`     // Size per chunk in bytes (default: 1MB)
    MinFileSize   int64 `mapstructure:"min_file_size"`  // Minimum file size to use multi-channel (default: 10MB)
}

// Add to WebRTCConfig
type WebRTCConfig struct {
    // Existing fields...
    MultiChannel MultiChannelConfig `mapstructure:"multi_channel"`
}
```

#### Usage Example
```bash
# Enable multi-channel transfer with 6 channels for large files
./yapfs send --file large_video.mp4 --multi-channel --max-channels 6

# Receiver automatically detects multi-channel transfer
./yapfs receive --dst ./downloads/
```

#### Progressive Enhancement
```go
func (s *SenderApp) shouldUseMultiChannel(fileSize int64) bool {
    return s.config.MultiChannel.Enabled && 
           fileSize >= s.config.MultiChannel.MinFileSize
}

func (s *SenderApp) Run(ctx context.Context, opts *SenderOptions) error {
    fileInfo, err := s.fileService.GetFileInfo(opts.FilePath)
    if err != nil {
        return err
    }
    
    if s.shouldUseMultiChannel(fileInfo.Size) {
        return s.runMultiChannelTransfer(ctx, opts, fileInfo)
    } else {
        return s.runSingleChannelTransfer(ctx, opts, fileInfo)
    }
}
```

#### Benefits
- **Increased throughput**: Parallel data channels can better utilize available bandwidth
- **Improved performance**: Especially beneficial for large files (>10MB)
- **Fault tolerance**: Single channel failure doesn't stop entire transfer
- **Scalable**: Number of channels can be configured based on network conditions
- **Backward compatibility**: Falls back to single channel for small files or when disabled

#### Implementation Considerations
- **Memory usage**: Need to balance chunk size vs. memory consumption
- **Network overhead**: More channels = more WebRTC overhead
- **Reassembly complexity**: Proper ordering and integrity verification required
- **Configuration tuning**: Optimal channel count depends on network conditions
- **Error handling**: Need to handle partial failures gracefully

## Implementation Roadmap

### Short Term (Next Release)
- [ ] File integrity verification (checksums)
- [ ] Basic path security and validation
- [ ] Cloudflare Worker SDP exchange
- [ ] Enhanced UI with progress bar and transfer statistics
- [ ] WebRTC connection retry and reliability improvements

### Medium Term
- [ ] Multiple data channels for concurrent file transfer
- [ ] Resume capability for interrupted transfers
- [ ] Peer authentication mechanisms
- [ ] Enhanced security hardening
- [ ] Advanced progress features (terminal width detection, color support)
- [ ] ICE server failover and connection monitoring

### Long Term
- [ ] Application-layer encryption
- [ ] Advanced security features (rate limiting, quotas)
- [ ] Rich terminal UI with multiple progress bars for concurrent transfers
- [ ] Advanced connection diagnostics and network analysis

## Contributing

When implementing these improvements:
3. **Follow existing code patterns** and architecture
4. **Add comprehensive tests** for security-critical features
5. **Update documentation** including CLI help and README

Each improvement should be implemented as a separate feature with appropriate configuration flags to maintain the tool's simplicity while adding power-user capabilities.ư