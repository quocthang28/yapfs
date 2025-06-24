package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"yapfs/internal/config"
	"yapfs/internal/processor"
	"yapfs/pkg/types"

	"github.com/pion/webrtc/v4"
)

// Channel represents a generic bidirectional WebRTC data channel
// It can be configured with different handlers for sender/receiver behavior
type Channel struct {
	ctx           context.Context
	config        *config.Config
	dataChannel   *webrtc.DataChannel
	dataProcessor *processor.DataProcessor
	handler       MessageHandler
	
	// Channel management
	readyCh         chan struct{}
	bufferControlCh chan struct{}
	
	// Message routing
	incomingMsgCh chan webrtc.DataChannelMessage
	outgoingMsgCh chan []byte
	
	// State management
	isReady    bool
	isClosed   bool
	closeMutex sync.RWMutex
	
	// Graceful shutdown
	shutdownOnce sync.Once
}

// NewChannel creates a new generic channel with the specified handler
func NewChannel(ctx context.Context, cfg *config.Config, handler MessageHandler) *Channel {
	return &Channel{
		ctx:             ctx,
		config:          cfg,
		handler:         handler,
		dataProcessor:   processor.NewDataProcessor(),
		readyCh:         make(chan struct{}),
		bufferControlCh: make(chan struct{}),
		incomingMsgCh:   make(chan webrtc.DataChannelMessage, 100),
		outgoingMsgCh:   make(chan []byte),
		isReady:         false,
		isClosed:        false,
	}
}

// CreateDataChannel creates and configures a WebRTC data channel for sending
func (c *Channel) CreateDataChannel(peerConn *webrtc.PeerConnection, label string) error {
	ordered := true
	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}

	dataChannel, err := peerConn.CreateDataChannel(label, options)
	if err != nil {
		return fmt.Errorf("failed to create data channel: %w", err)
	}

	c.dataChannel = dataChannel
	c.setupDataChannelHandlers()
	
	return nil
}

// SetupReceiverDataChannel configures the channel to receive an incoming data channel
func (c *Channel) SetupReceiverDataChannel(peerConn *webrtc.PeerConnection) error {
	peerConn.OnDataChannel(func(dataChannel *webrtc.DataChannel) {
		c.dataChannel = dataChannel
		log.Printf("Received data channel: %s-%d", dataChannel.Label(), dataChannel.ID())
		c.setupDataChannelHandlers()
	})
	
	return nil
}

// setupDataChannelHandlers configures WebRTC data channel event handlers
func (c *Channel) setupDataChannelHandlers() {
	c.dataChannel.OnOpen(func() {
		log.Printf("Data channel opened: %s-%d", c.dataChannel.Label(), c.dataChannel.ID())
		c.markReady()
	})

	c.dataChannel.OnClose(func() {
		log.Printf("Data channel closed")
		c.handleClose()
	})

	c.dataChannel.OnError(func(err error) {
		log.Printf("Data channel error: %v", err)
		c.handleError(err)
	})

	c.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		select {
		case c.incomingMsgCh <- msg:
		case <-c.ctx.Done():
			return
		}
	})

	// Set up flow control
	c.dataChannel.SetBufferedAmountLowThreshold(c.config.WebRTC.BufferedAmountLowThreshold)
	c.dataChannel.OnBufferedAmountLow(func() {
		select {
		case c.bufferControlCh <- struct{}{}:
		default:
		}
	})
}

// StartMessageLoop starts the main message processing loop
// Returns a progress channel for monitoring transfer progress
func (c *Channel) StartMessageLoop() <-chan types.ProgressUpdate {
	progressCh := make(chan types.ProgressUpdate, 50)

	go func() {
		defer close(progressCh)
		defer c.shutdown()

		// Wait for channel to be ready
		if err := c.waitForReady(); err != nil {
			log.Printf("Error waiting for channel ready: %v", err)
			return
		}

		// Start message processing loops first
		var wg sync.WaitGroup
		
		// Incoming message processing
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.processIncomingMessages(progressCh)
		}()

		// Outgoing message processing
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.processOutgoingMessages()
		}()

		// Give message loops a moment to start before notifying handler
		// This ensures outgoing messages can be processed immediately
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Notify handler that channel is ready
		if err := c.handler.OnChannelReady(); err != nil {
			log.Printf("Handler OnChannelReady error: %v", err)
			return
		}

		// Wait for completion or cancellation
		wg.Wait()
	}()

	return progressCh
}

// SendMessage queues a message for sending
func (c *Channel) SendMessage(msgType MessageType, payload []byte) error {
	if c.IsClosed() {
		return fmt.Errorf("channel is closed")
	}

	msg := Message{
		Type:    msgType,
		Payload: payload,
	}
	data, err := SerializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	select {
	case c.outgoingMsgCh <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("channel context cancelled")
	}
}


// processIncomingMessages handles incoming WebRTC messages
func (c *Channel) processIncomingMessages(progressCh chan<- types.ProgressUpdate) {
	for {
		select {
		case msg := <-c.incomingMsgCh:
			if err := c.handleIncomingMessage(msg, progressCh); err != nil {
				log.Printf("Error handling incoming message: %v", err)
				c.handleError(err)
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// handleIncomingMessage processes a single incoming message
func (c *Channel) handleIncomingMessage(msg webrtc.DataChannelMessage, progressCh chan<- types.ProgressUpdate) error {
	// Parse structured message
	parsedMsg, err := DeserializeMessage(msg.Data)
	if err != nil {
		return fmt.Errorf("failed to deserialize message: %w", err)
	}

	return c.handler.HandleMessage(parsedMsg, progressCh)
}

// processOutgoingMessages handles outgoing message queue
func (c *Channel) processOutgoingMessages() {
	for {
		select {
		case data := <-c.outgoingMsgCh:
			if err := c.sendRawData(data); err != nil {
				log.Printf("Error sending message: %v", err)
				c.handleError(err)
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// sendRawData sends raw data through the WebRTC data channel with flow control
func (c *Channel) sendRawData(data []byte) error {
	if c.IsClosed() {
		return fmt.Errorf("channel is closed")
	}

	// Handle flow control
	if err := c.handleFlowControl(); err != nil {
		return err
	}

	// Send the data
	err := c.dataChannel.Send(data)
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}

	return nil
}

// handleFlowControl manages WebRTC buffer flow control
func (c *Channel) handleFlowControl() error {
	if c.dataChannel.BufferedAmount() > c.config.WebRTC.MaxBufferedAmount {
		select {
		case <-c.bufferControlCh:
			return nil
		case <-c.ctx.Done():
			return fmt.Errorf("channel cancelled during flow control")
		case <-time.After(30 * time.Second):
			return fmt.Errorf("flow control timeout - WebRTC channel may be dead")
		}
	}
	return nil
}

// waitForReady waits for the data channel to be ready
func (c *Channel) waitForReady() error {
	if c.isReady {
		return nil
	}

	select {
	case <-c.readyCh:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("cancelled while waiting for channel ready: %w", c.ctx.Err())
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for channel ready")
	}
}

// markReady marks the channel as ready and notifies waiters
func (c *Channel) markReady() {
	if !c.isReady {
		c.isReady = true
		close(c.readyCh)
	}
}

// handleClose handles channel close events
func (c *Channel) handleClose() {
	c.closeMutex.Lock()
	if c.isClosed {
		c.closeMutex.Unlock()
		return // Already handled
	}
	c.isClosed = true
	c.closeMutex.Unlock()
	
	c.handler.OnChannelClosed()
	c.shutdown()
}

// handleError handles channel error events
func (c *Channel) handleError(err error) {
	c.closeMutex.Lock()
	if c.isClosed {
		c.closeMutex.Unlock()
		return // Already handled
	}
	c.isClosed = true
	c.closeMutex.Unlock()
	
	c.handler.OnChannelError(err)
	c.shutdown()
}

// IsClosed returns whether the channel is closed
func (c *Channel) IsClosed() bool {
	c.closeMutex.RLock()
	defer c.closeMutex.RUnlock()
	return c.isClosed
}

// Close gracefully closes the data channel
func (c *Channel) Close() error {
	c.closeMutex.Lock()
	if c.isClosed {
		c.closeMutex.Unlock()
		return nil // Already closed
	}
	c.isClosed = true
	c.closeMutex.Unlock()

	if c.dataChannel != nil {
		// Only attempt graceful close if the channel is still in a valid state
		if c.dataChannel.ReadyState() == webrtc.DataChannelStateOpen {
			err := c.dataChannel.GracefulClose()
			if err != nil {
				log.Printf("Error during graceful close: %v", err)
			}
		}
	}

	c.shutdown()
	return nil
}

// shutdown performs cleanup and notifies components
func (c *Channel) shutdown() {
	c.shutdownOnce.Do(func() {
		c.closeMutex.Lock()
		c.isClosed = true
		c.closeMutex.Unlock()

		if c.dataProcessor != nil {
			c.dataProcessor.Close()
		}

		// Close channels if they haven't been closed yet
		select {
		case <-c.readyCh:
		default:
			close(c.readyCh)
		}
	})
}