# Future Improvements for YAPFS

## Acknowledgement Protocol Enhancement

### Current Implementation Analysis

The current file transfer protocol is essentially one-way communication from sender to receiver without proper acknowledgements. This creates potential reliability issues.

### Current Flow

**Start of Transfer:**
- Sender: Creates data channel → waits for open → sends metadata immediately
- Receiver: Sets up handler → receives data channel → receives metadata → prepares file

**During Transfer:**
- Sender: Sends data chunks with flow control
- Receiver: Receives chunks → writes to file

**End of Transfer:**
- Sender: Sends "EOF" → closes channel gracefully
- Receiver: Processes EOF → finishes file → reports completion

### Missing Acknowledgements

**1. Receiver Readiness** (`internal/transport/receiver_channel.go:86`)
- Sender starts sending once data channel opens, but receiver may not be ready
- No confirmation that receiver successfully set up file handlers

**2. Metadata Acknowledgement** (`internal/transport/receiver_channel.go:108-140`)
- Sender sends metadata but gets no confirmation of successful receipt/processing
- Receiver could fail file preparation, but sender continues sending data

**3. Transfer Completion Acknowledgement** (`internal/transport/receiver_channel.go:169-177`)
- Sender sends EOF and closes channel without knowing if receiver succeeded
- No confirmation of checksum verification or successful file completion

**4. Error Propagation**
- Receiver errors (write failures, disk full, etc.) aren't communicated back to sender
- Sender may complete "successfully" while receiver actually failed

### Proposed Enhancement: Bidirectional Acknowledgement Protocol

Add these acknowledgement messages to create a robust handshake protocol:

#### New Message Types
- `READY` - Receiver confirms it's ready to receive file data
- `METADATA_ACK` - Receiver confirms successful metadata processing  
- `TRANSFER_COMPLETE` - Receiver confirms successful file completion with checksum
- `ERROR:<reason>` - Receiver reports errors to sender

#### Enhanced Flow

**Start of Transfer:**
1. Sender: Creates data channel → waits for open
2. Receiver: Sets up handler → receives data channel → sends `READY`
3. Sender: Receives `READY` → sends metadata
4. Receiver: Receives metadata → prepares file → sends `METADATA_ACK` or `ERROR`
5. Sender: Receives `METADATA_ACK` → starts file data transfer

**During Transfer:**
- Sender: Sends data chunks with flow control
- Receiver: Receives chunks → writes to file
- Optional: Periodic acknowledgements for large files

**End of Transfer:**
1. Sender: Sends "EOF"
2. Receiver: Processes EOF → verifies checksum → sends `TRANSFER_COMPLETE` or `ERROR`
3. Sender: Receives confirmation → closes channel gracefully

#### Implementation Changes Required

**Sender Side (`internal/transport/sender_channel.go`):**
- Add message handler for incoming acknowledgements
- Implement state machine for protocol phases
- Add timeout handling for acknowledgements
- Modify `sendMetadataPhase()` to wait for `METADATA_ACK`
- Modify `sendEOF()` to wait for `TRANSFER_COMPLETE`

**Receiver Side (`internal/transport/receiver_channel.go`):**
- Send `READY` message when data channel opens
- Send `METADATA_ACK` after successful file preparation
- Send `TRANSFER_COMPLETE` after successful checksum verification
- Send `ERROR` messages for any failure conditions

#### Benefits
- Guaranteed receiver readiness before transfer starts
- Early detection of receiver-side errors
- Confirmation of successful transfer completion
- Better error reporting and debugging
- More robust handling of edge cases (disk full, permission errors, etc.)

#### Backward Compatibility
- New protocol could be negotiated during initial handshake
- Fallback to current one-way protocol for older versions
- Version field in metadata to indicate protocol capabilities

### Implementation Priority
This enhancement would significantly improve transfer reliability and should be considered for the next major version of YAPFS.