 Acknowledgment Flow Design

  1. Add acknowledgment message types:
  // In sender_channel.go - after EOF
  const (
      MSG_EOF = "EOF"
      MSG_ACK = "ACK"
  )

  2. Modify receiver to send ACK after EOF:
  // In receiver_channel.go handleEOFMessage (line 167)
  func (r *ReceiverChannel) handleEOFMessage(msg webrtc.DataChannelMessage, ctx *MessageHandlerContext) {
      // Existing EOF processing...
      totalBytes, err := r.dataProcessor.FinishReceiving()
      if err != nil {
          log.Printf("Error processing EOF signal: %v", err)
          return
      }

      // Send ACK back to sender
      err = r.dataChannel.Send([]byte(MSG_ACK))
      if err != nil {
          log.Printf("Error sending ACK: %v", err)
      }

      log.Printf("File transfer complete: %d bytes received, ACK sent", totalBytes)
      // Rest of existing code...
  }

  3. Modify sender to wait for ACK:
  // In sender_channel.go sendEOFPhase (line 214)
  func (s *SenderChannel) sendEOFPhase() error {
      // Send EOF marker
      err := s.dataChannel.Send([]byte(MSG_EOF))
      if err != nil {
          return fmt.Errorf("error sending EOF: %v", err)
      }

      // Set up ACK listener before waiting
      ackReceived := make(chan bool, 1)
      s.dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
          if string(msg.Data) == MSG_ACK {
              ackReceived <- true
          }
      })

      // Wait for ACK with timeout
      select {
      case <-ackReceived:
          log.Printf("Received ACK from receiver, closing gracefully")
      case <-time.After(10 * time.Second):
          log.Printf("ACK timeout, proceeding to close")
      case <-s.ctx.Done():
          return fmt.Errorf("cancelled while waiting for ACK: %v", s.ctx.Err())
      }

      // Now safe to close
      err = s.dataChannel.GracefulClose()
      if err != nil {
          return fmt.Errorf("error closing channel: %v", err)
      }

      return nil
  }

  Benefits:
  - Ensures receiver processed all data before sender closes
  - Timeout prevents hanging on network issues
  - Maintains data integrity for file transfers
  - Minimal overhead (single ACK message)