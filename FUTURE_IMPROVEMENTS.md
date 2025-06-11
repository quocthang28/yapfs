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

## Implementation Roadmap

### Short Term (Next Release)
- [ ] File integrity verification (checksums)
- [ ] Basic path security and validation
- [ ] Cloudflare Worker SDP exchange
- [ ] Enhanced UI with progress bar and transfer statistics

### Medium Term
- [ ] Resume capability for interrupted transfers
- [ ] Peer authentication mechanisms
- [ ] Enhanced security hardening
- [ ] Advanced progress features (terminal width detection, color support)

### Long Term
- [ ] Application-layer encryption
- [ ] Advanced security features (rate limiting, quotas)
- [ ] Rich terminal UI with multiple progress bars for concurrent transfers

## Contributing

When implementing these improvements:
3. **Follow existing code patterns** and architecture
4. **Add comprehensive tests** for security-critical features
5. **Update documentation** including CLI help and README

Each improvement should be implemented as a separate feature with appropriate configuration flags to maintain the tool's simplicity while adding power-user capabilities.ư