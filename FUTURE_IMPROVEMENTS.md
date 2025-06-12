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

## 3. Transfer Progress Monitoring and Reporting

### Current State
- Basic console output with minimal transfer feedback
- No visual progress indication during file transfers
- Simple byte counting without throughput statistics
- No ETA or completion progress information

### Proposed Progress System

#### Comprehensive Progress Monitoring Service
- **Transfer state tracking**: Monitor bytes transferred, throughput, and timing
- **Progress calculation**: Real-time percentage completion and ETA estimation
- **Multi-UI support**: Interface-based design supporting console, web, or GUI displays
- **Thread-safe metrics**: Atomic operations for concurrent transfer monitoring

#### Core Progress Architecture
```go
// Service for monitoring and calculating transfer progress
type ProgressMonitor struct {
    startTime    time.Time
    totalBytes   int64
    currentBytes *uint64  // atomic counter
    reporter     ProgressReporter
}

// Interface for different UI implementations
type ProgressReporter interface {
    UpdateProgress(percentage float64, throughputMbps float64, bytesTransferred int64, totalBytes int64)
    ReportCompletion(message string)
    ReportError(err error)
}

// Console implementation with ASCII progress bar
type ConsoleProgressReporter struct {
    ui *ConsoleUI
}

// Real-time throughput and progress calculation
type TransferMetrics struct {
    BytesTransferred int64
    TotalBytes       int64
    ThroughputMbps   float64
    Percentage       float64
    ElapsedTime      time.Duration
    ETADuration      time.Duration
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

## 4. Automated SDP Exchange

### Current Limitations
- Manual copy/paste of SDP is error-prone and cumbersome
- No automation possible for SDP exchange
- Difficult to use programmatically
- Poor user experience for non-technical users

### Proposed Solution: Firebase Realtime Database SDP Exchange

#### Architecture Overview
```
Sender                    Firebase Realtime DB                 Receiver
  |                              |                               |
  |---> Write /sessions/{id}/offer                              |
  |     (SDP offer)              |                               |
  |                              |                               |
  |                              |<--- Listen /sessions/{id} <---|
  |                              |     (real-time updates)       |
  |                              |                               |
  |<--- Listen /sessions/{id} ---|---> Write /sessions/{id}/answer
  |     (real-time updates)      |     (SDP answer)              |
```

#### Firebase Database Structure
```json
{
  "sessions": {
    "{sessionId}": {
      "offer": "base64_encoded_sdp_offer",
      "answer": "base64_encoded_sdp_answer",
      "candidates": {
        "sender": {
          "{timestamp}": {
            "candidate": "ice_candidate_data",
            "sdpMid": "0",
            "sdpMLineIndex": 0
          }
        },
        "receiver": {
          "{timestamp}": {
            "candidate": "ice_candidate_data", 
            "sdpMid": "0",
            "sdpMLineIndex": 0
          }
        }
      },
      "metadata": {
        "created": 1640995200000,
        "expires": 1640995500000,
        "status": "waiting"
      }
    }
  }
}
```

#### Firebase Security Rules
```javascript
{
  "rules": {
    "sessions": {
      "$sessionId": {
        // Allow read/write access to anyone with the session ID
        ".read": true,
        ".write": true,
        
        // Automatically delete sessions after 5 minutes
        ".validate": "newData.child('metadata').child('expires').val() > now",
        
        // Validate session structure
        "offer": {
          ".validate": "newData.isString() && newData.val().length > 0"
        },
        "answer": {
          ".validate": "newData.isString() && newData.val().length > 0"
        },
        "candidates": {
          "$role": {
            "$timestamp": {
              ".validate": "newData.hasChildren(['candidate'])"
            }
          }
        },
        "metadata": {
          ".validate": "newData.hasChildren(['created', 'expires'])"
        }
      }
    }
  }
}
```

#### YAPFS Firebase Integration
```go
import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    firebase "firebase.google.com/go/v4"
    "firebase.google.com/go/v4/db"
    "google.golang.org/api/option"
)

type FirebaseSignaling struct {
    client    *db.Client
    sessionID string
    timeout   time.Duration
}

type SessionData struct {
    Offer      string                 `json:"offer,omitempty"`
    Answer     string                 `json:"answer,omitempty"`
    Candidates map[string]interface{} `json:"candidates,omitempty"`
    Metadata   SessionMetadata        `json:"metadata"`
}

type SessionMetadata struct {
    Created int64  `json:"created"`
    Expires int64  `json:"expires"`
    Status  string `json:"status"`
}

func NewFirebaseSignaling(projectID, databaseURL, sessionID string) (*FirebaseSignaling, error) {
    ctx := context.Background()
    
    // Initialize Firebase app
    conf := &firebase.Config{
        DatabaseURL: databaseURL,
        ProjectID:   projectID,
    }
    
    app, err := firebase.NewApp(ctx, conf, option.WithCredentialsFile("path/to/serviceAccount.json"))
    if err != nil {
        return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
    }
    
    // Get Realtime Database client
    client, err := app.Database(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize Firebase database: %w", err)
    }
    
    return &FirebaseSignaling{
        client:    client,
        sessionID: sessionID,
        timeout:   30 * time.Second,
    }, nil
}

func (f *FirebaseSignaling) CreateSession(ctx context.Context) error {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s", f.sessionID))
    
    sessionData := SessionData{
        Metadata: SessionMetadata{
            Created: time.Now().Unix(),
            Expires: time.Now().Add(5 * time.Minute).Unix(),
            Status:  "waiting",
        },
    }
    
    return ref.Set(ctx, sessionData)
}

func (f *FirebaseSignaling) PublishOffer(ctx context.Context, offer *webrtc.SessionDescription) error {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s/offer", f.sessionID))
    
    // Encode SDP to base64
    encodedOffer := base64.StdEncoding.EncodeToString([]byte(offer.SDP))
    
    return ref.Set(ctx, encodedOffer)
}

func (f *FirebaseSignaling) ListenForAnswer(ctx context.Context) (*webrtc.SessionDescription, error) {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s/answer", f.sessionID))
    
    // Create channel for real-time updates
    answerCh := make(chan string, 1)
    errCh := make(chan error, 1)
    
    // Listen for answer updates
    listener := ref.Listen(ctx, func(snapshot db.DataSnapshot) {
        var answer string
        if err := snapshot.Unmarshal(&answer); err != nil {
            errCh <- err
            return
        }
        
        if answer != "" {
            answerCh <- answer
        }
    })
    defer listener.Close()
    
    // Wait for answer or timeout
    select {
    case answer := <-answerCh:
        // Decode base64 SDP
        decodedSDP, err := base64.StdEncoding.DecodeString(answer)
        if err != nil {
            return nil, fmt.Errorf("failed to decode answer SDP: %w", err)
        }
        
        return &webrtc.SessionDescription{
            Type: webrtc.SDPTypeAnswer,
            SDP:  string(decodedSDP),
        }, nil
        
    case err := <-errCh:
        return nil, err
        
    case <-time.After(f.timeout):
        return nil, fmt.Errorf("timeout waiting for answer")
        
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (f *FirebaseSignaling) PublishAnswer(ctx context.Context, answer *webrtc.SessionDescription) error {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s/answer", f.sessionID))
    
    // Encode SDP to base64
    encodedAnswer := base64.StdEncoding.EncodeToString([]byte(answer.SDP))
    
    return ref.Set(ctx, encodedAnswer)
}

func (f *FirebaseSignaling) ListenForOffer(ctx context.Context) (*webrtc.SessionDescription, error) {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s/offer", f.sessionID))
    
    // Create channel for real-time updates
    offerCh := make(chan string, 1)
    errCh := make(chan error, 1)
    
    // Listen for offer updates
    listener := ref.Listen(ctx, func(snapshot db.DataSnapshot) {
        var offer string
        if err := snapshot.Unmarshal(&offer); err != nil {
            errCh <- err
            return
        }
        
        if offer != "" {
            offerCh <- offer
        }
    })
    defer listener.Close()
    
    // Wait for offer or timeout
    select {
    case offer := <-offerCh:
        // Decode base64 SDP
        decodedSDP, err := base64.StdEncoding.DecodeString(offer)
        if err != nil {
            return nil, fmt.Errorf("failed to decode offer SDP: %w", err)
        }
        
        return &webrtc.SessionDescription{
            Type: webrtc.SDPTypeOffer,
            SDP:  string(decodedSDP),
        }, nil
        
    case err := <-errCh:
        return nil, err
        
    case <-time.After(f.timeout):
        return nil, fmt.Errorf("timeout waiting for offer")
        
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// Real-time ICE candidate exchange
func (f *FirebaseSignaling) SetupTrickleICE(ctx context.Context, pc *webrtc.PeerConnection, role string) {
    // Listen for incoming candidates
    go f.listenForCandidates(ctx, pc, role)
    
    // Send outgoing candidates
    pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
        if candidate == nil {
            return
        }
        
        f.publishCandidate(ctx, candidate, role)
    })
}

func (f *FirebaseSignaling) listenForCandidates(ctx context.Context, pc *webrtc.PeerConnection, myRole string) {
    // Listen to the other peer's candidates
    otherRole := "receiver"
    if myRole == "receiver" {
        otherRole = "sender"
    }
    
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s/candidates/%s", f.sessionID, otherRole))
    
    listener := ref.Listen(ctx, func(snapshot db.DataSnapshot) {
        var candidates map[string]interface{}
        if err := snapshot.Unmarshal(&candidates); err != nil {
            return
        }
        
        for _, candidateData := range candidates {
            if candidateMap, ok := candidateData.(map[string]interface{}); ok {
                candidate := webrtc.ICECandidateInit{
                    Candidate: candidateMap["candidate"].(string),
                }
                
                if sdpMid, ok := candidateMap["sdpMid"].(string); ok {
                    candidate.SDPMid = &sdpMid
                }
                
                if sdpMLineIndex, ok := candidateMap["sdpMLineIndex"].(float64); ok {
                    idx := uint16(sdpMLineIndex)
                    candidate.SDPMLineIndex = &idx
                }
                
                pc.AddICECandidate(candidate)
            }
        }
    })
    
    // Clean up listener when context is done
    go func() {
        <-ctx.Done()
        listener.Close()
    }()
}

func (f *FirebaseSignaling) publishCandidate(ctx context.Context, candidate *webrtc.ICECandidate, role string) error {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s/candidates/%s", f.sessionID, role))
    
    candidateData := map[string]interface{}{
        "candidate": candidate.Candidate,
        "sdpMid":    candidate.SDPMid,
        "sdpMLineIndex": candidate.SDPMLineIndex,
    }
    
    // Use timestamp as key for ordering
    timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
    
    return ref.Child(timestamp).Set(ctx, candidateData)
}

// Cleanup session after transfer
func (f *FirebaseSignaling) CleanupSession(ctx context.Context) error {
    ref := f.client.NewRef(fmt.Sprintf("sessions/%s", f.sessionID))
    return ref.Delete(ctx)
}
```

#### Configuration Integration
```go
type FirebaseConfig struct {
    ProjectID     string `mapstructure:"project_id"`
    DatabaseURL   string `mapstructure:"database_url"`
    CredentialsPath string `mapstructure:"credentials_path"`
    Timeout       time.Duration `mapstructure:"timeout"`
}

// Add to main config
type Config struct {
    // Existing fields...
    Firebase FirebaseConfig `mapstructure:"firebase"`
}
```

#### CLI Integration
```go
// Add to send command flags
type SendFlags struct {
    FilePath  string // Existing
    SessionID string // New: --session flag
    Firebase  bool   // New: --firebase flag
}

// Sender app integration
func (s *SenderApp) Run(ctx context.Context, opts *SenderOptions) error {
    if opts.SessionID != "" {
        // Use Firebase signaling
        firebaseSignaling, err := transport.NewFirebaseSignaling(
            s.config.Firebase.ProjectID,
            s.config.Firebase.DatabaseURL,
            opts.SessionID,
        )
        if err != nil {
            return fmt.Errorf("failed to initialize Firebase signaling: %w", err)
        }
        
        return s.runWithFirebaseSignaling(ctx, opts, firebaseSignaling)
    }
    
    // Fall back to manual SDP exchange
    return s.runWithManualSignaling(ctx, opts)
}
```

#### Usage Flow
```bash
# Install and configure Firebase
go get firebase.google.com/go/v4
export GOOGLE_APPLICATION_CREDENTIALS="path/to/serviceAccount.json"

# Sender creates session and shares session ID
./yapfs send --file /path/to/video.mp4 --session abc123 --firebase
Session ID: abc123
Share this session ID with the receiver
Waiting for receiver to join...

# Receiver joins using session ID
./yapfs receive --dst ./downloads/ --session abc123 --firebase  
Joined session: abc123
Waiting for file transfer to begin...
Connected! Receiving: video.mp4 (1.2GB)
[=============>      ] 67.3% 823MB/1.2GB 15.2MB/s ETA: 26s

# Alternative: Use configuration file
./yapfs send --file video.mp4 --session abc123 --config firebase-config.yaml
```

#### Firebase Configuration File Example
```yaml
# ~/.yapfs.yaml or firebase-config.yaml
firebase:
  project_id: "yapfs-signaling"
  database_url: "https://yapfs-signaling-default-rtdb.firebaseio.com/"
  credentials_path: "/path/to/serviceAccount.json"
  timeout: "30s"

webrtc:
  ice_servers:
    - urls: ["stun:stun.l.google.com:19302"]
```

#### Benefits
- **Real-time updates**: Firebase Realtime Database provides instant SDP and candidate exchange
- **No server maintenance**: Managed service with automatic scaling and reliability
- **Global CDN**: Firebase's global infrastructure ensures low latency worldwide
- **Built-in security**: Firebase security rules control access and data validation
- **Automatic cleanup**: TTL-based session expiration prevents data buildup
- **Trickle ICE support**: Real-time candidate exchange for faster connection establishment
- **Simple setup**: No custom server deployment required
- **Free tier available**: Firebase offers generous free quotas for small usage

#### Security Considerations
- **Session ID entropy**: Use cryptographically secure random session IDs (32+ characters)
- **Firebase security rules**: Implement proper read/write rules and data validation
- **Service account security**: Secure storage of Firebase service account credentials
- **Data encryption**: SDP data is base64 encoded and automatically encrypted in transit
- **Session expiration**: Automatic cleanup after 5 minutes prevents stale data
- **No file content**: Only signaling data is stored, never actual file content
- **Rate limiting**: Firebase provides built-in rate limiting and abuse protection

#### Setup Instructions

##### 1. Firebase Project Setup
```bash
# Create Firebase project
# 1. Go to https://console.firebase.google.com/
# 2. Create new project "yapfs-signaling"
# 3. Enable Realtime Database
# 4. Set database rules (see Security Rules section above)
# 5. Generate service account key
```

##### 2. Go Dependencies
```bash
go get firebase.google.com/go/v4
go get firebase.google.com/go/v4/db
go get google.golang.org/api/option
```

##### 3. Environment Setup
```bash
# Set service account credentials
export GOOGLE_APPLICATION_CREDENTIALS="path/to/serviceAccount.json"

# Or configure in YAPFS config file
echo "firebase:
  project_id: yapfs-signaling
  database_url: https://yapfs-signaling-default-rtdb.firebaseio.com/
  credentials_path: ./serviceAccount.json
  timeout: 30s" > ~/.yapfs.yaml
```

#### Implementation Priority

##### Phase 1: Firebase Project Setup and Basic Integration (1-2 weeks)
1. **Firebase project creation and configuration**
   - Create Firebase project with Realtime Database
   - Configure security rules for session management
   - Generate and secure service account credentials

2. **Go Firebase SDK integration**
   - Add Firebase dependencies to go.mod
   - Implement `FirebaseSignaling` service with basic SDP exchange
   - Add Firebase configuration to YAPFS config system

3. **CLI flag integration**
   - Add `--session` and `--firebase` flags to send/receive commands
   - Implement session ID generation and validation
   - Add fallback to manual SDP exchange when Firebase is not configured

##### Phase 2: Real-time ICE Candidate Exchange (1 week)
1. **Trickle ICE implementation**
   - Add real-time ICE candidate publishing and listening
   - Implement proper candidate role separation (sender/receiver)
   - Add timeout handling for candidate exchange

2. **Connection optimization**
   - Optimize connection establishment speed with trickle ICE
   - Add connection state monitoring and error handling
   - Implement proper session cleanup after transfers

##### Phase 3: Enhanced Security and Error Handling (1 week)
1. **Security improvements**
   - Implement cryptographically secure session ID generation
   - Add proper error handling for Firebase connection failures
   - Implement session expiration and automatic cleanup

2. **User experience enhancements**
   - Add clear status messages for Firebase connection states
   - Implement retry logic for temporary Firebase connectivity issues
   - Add configuration validation and helpful error messages

##### Phase 4: Advanced Features and Optimization (Optional)
1. **Performance optimizations**
   - Implement connection pooling for Firebase clients
   - Add metrics and monitoring for Firebase usage
   - Optimize data structures for minimal Firebase usage

2. **Extended functionality**
   - Add optional session metadata (file size, transfer speed estimates)
   - Implement session discovery (list active sessions)
   - Add optional web UI for session management

#### Integration with Existing YAPFS Architecture

The Firebase signaling integration follows YAPFS's existing service-oriented architecture:

```go
// Service instantiation in cmd/root.go
func createServices() (...) {
    // Existing services
    peerService := transport.NewPeerService(cfg, stateHandler)
    signalingService := transport.NewSignalingService()
    
    // New Firebase signaling service (optional)
    var firebaseSignaling *transport.FirebaseSignaling
    if cfg.Firebase.ProjectID != "" {
        firebaseSignaling = transport.NewFirebaseSignaling(
            cfg.Firebase.ProjectID,
            cfg.Firebase.DatabaseURL,
            sessionID,
        )
    }
    
    return peerService, signalingService, firebaseSignaling, consoleUI, dataChannelService
}

// App layer integration maintains separation of concerns
func (s *SenderApp) Run(ctx context.Context, opts *SenderOptions) error {
    if opts.SessionID != "" && s.firebaseSignaling != nil {
        return s.runWithFirebaseSignaling(ctx, opts)
    }
    return s.runWithManualSignaling(ctx, opts)
}
```

This approach ensures:
- **Zero breaking changes**: Existing manual SDP exchange remains default
- **Progressive enhancement**: Firebase is opt-in via configuration
- **Service isolation**: Firebase signaling is a separate service with clear interfaces
- **Testability**: Services can be easily mocked and tested independently

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

## 7. Advanced Metadata and Transfer Control

#### Enhanced Metadata Exchange Protocol
```go
type MetadataExchangeProtocol struct {
    Type    string       `json:"type"`    // "metadata", "accept", "reject"
    Data    FileMetadata `json:"data,omitempty"`
    Message string       `json:"message,omitempty"` // Rejection reason or custom message
}

// Sender waits for receiver confirmation
func (s *Sender) WaitForReceiverConfirmation(timeout time.Duration) error {
    select {
    case response := <-s.confirmationCh:
        if response.Type == "reject" {
            return fmt.Errorf("receiver rejected file: %s", response.Message)
        }
        return nil
    case <-time.After(timeout):
        return fmt.Errorf("receiver confirmation timeout")
    }
}
```

#### Timeout Handling and Graceful Degradation
```go
type TimeoutConfig struct {
    MetadataExchange time.Duration // 30 seconds
    UserConfirmation time.Duration // 60 seconds
    ConnectionSetup  time.Duration // 45 seconds
}

func (r *Receiver) HandleTimeouts(config TimeoutConfig) {
    // Graceful handling of various timeout scenarios
    // - Metadata exchange timeout: retry or fallback to basic mode
    // - User confirmation timeout: default accept/reject behavior
    // - Connection timeout: retry with exponential backoff
}
```

### Implementation Notes
- **Optional feature**: Can be enabled/disabled via configuration
- **Backward compatibility**: Maintains current auto-accept behavior as default
- **Configurable timeouts**: All timeout values should be configurable
- **Clear error messages**: Provide helpful feedback for timeout scenarios

## Implementation Roadmap

### Short Term (Next Release)
- [ ] File integrity verification (checksums)
- [ ] Basic path security and validation
- [ ] Automated SDP exchange
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