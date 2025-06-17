# YAPFS - Yet Another P2P File Sharing

A secure peer-to-peer file sharing utility built with WebRTC data channels. Transfer files directly between machines without a central server.

## Quick Start

```bash
# Build
go build -o yapfs

# Send a file
./yapfs send --file /path/to/your/file

# Receive a file  
./yapfs receive --dst /path/to/save/file
```

## How It Works

1. **Start sender**: Run `yapfs send` with your file
2. **Start receiver**: Run `yapfs receive` with destination path
3. **Exchange session ID**: Sender sends the generated session ID to the receiver using an external communication method.
4. **Transfer**: Files transfer directly via WebRTC with progress monitoring

## Features

- **Direct P2P transfer** - No intermediary servers required
- **Secure WebRTC** - Encrypted data channels with ICE connectivity
- **Progress monitoring** - Real-time throughput and completion tracking
- **Flow control** - Intelligent buffering prevents network congestion
- **Large file support** - Streaming chunks with constant memory usage
- **Cross-platform** - Works on Linux, macOS, and Windows

## Configuration

### Available Configuration Options

YAPFS supports configuration via JSON files. See `example-config.json` for a complete template.

#### WebRTC Settings (`webrtc`)

- **`ice_servers`** - Array of STUN/TURN servers for NAT traversal
  - Default: `[{"urls": ["stun:stun.l.google.com:19302"]}]`
  - Multiple servers can be specified for redundancy

- **`packet_size`** - Size of each file chunk in bytes
  - Default: `1024` (1 KB)
  - Smaller values reduce memory usage but may decrease throughput
  - Range: 512-8192 bytes recommended

- **`max_buffered_amount`** - Maximum WebRTC send buffer size in bytes
  - Default: `1048576` (1 MB)
  - Higher values allow more data buffering but use more memory
  - Triggers flow control when reached to prevent overwhelming the connection

- **`buffered_amount_low_threshold`** - Resume transmission threshold in bytes
  - Default: `524288` (512 KB)
  - Must be less than `max_buffered_amount`
  - Flow control resumes sending when buffer drops below this level

#### Firebase Settings (`firebase`)

- **`project_id`** - Your Firebase project identifier
- **`database_url`** - Firebase Realtime Database URL
- **`credentials_path`** - Path to Firebase service account JSON key file

## Firebase Setup

Firebase Realtime Database is used for automated SDP exchange, eliminating the need for manual copy/paste of connection details.

### 1. Create Firebase Project
1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Click "Create a project" or "Add project"
3. Enter project name (e.g., "yapfs-transfers")
4. Configure Google Analytics (optional)
5. Click "Create project"

### 2. Enable Realtime Database
1. In your Firebase project, go to "Build" → "Realtime Database"
2. Click "Create Database"
3. Choose location (select closest to your region)
4. Start in **test mode** for now (we'll secure it later)
5. Click "Done"

### 3. Generate Service Account Key
1. Go to "Project Settings" (gear icon) → "Service accounts"
2. Click "Generate new private key"
3. Click "Generate key" to download the JSON file
4. Save the file securely (e.g., `~/.yapfs-firebase-key.json`)

### 4. Add Firebase to config
Add Firebase settings to your config file (`config.json`). See `example-config.json` for a complete configuration template:

### 5. Database Security Rules (Production)
For production use, update your Realtime Database rules:

```json
{
  "rules": {
    "sessions": {
      "$sessionId": {
        ".write": true,
        ".read": true,
        ".validate": "newData.hasChildren(['offer', 'answer']) || newData.hasChild('offer')",
        "offer": {
          ".validate": "newData.isString()"
        },
        "answer": {
          ".validate": "newData.isString()"
        }
      }
    }
  }
}
```
