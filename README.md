# data-channels-flow-control
This example demonstrates how to use the following property / methods.

* func (d *DataChannel) BufferedAmount() uint64
* func (d *DataChannel) SetBufferedAmountLowThreshold(th uint64)
* func (d *DataChannel) BufferedAmountLowThreshold() uint64
* func (d *DataChannel) OnBufferedAmountLow(f func())

These methods are equivalent to that of JavaScript WebRTC API.
See https://developer.mozilla.org/en-US/docs/Web/API/RTCDataChannel for more details.

## When do we need it?
Send or SendText methods are called on DataChannel to send data to the connected peer.
The methods return immediately, but it does not mean the data was actually sent onto
the wire. Instead, it is queued in a buffer until it actually gets sent out to the wire.

When you have a large amount of data to send, it is an application's responsibility to
control the buffered amount in order not to indefinitely grow the buffer size to eventually
exhaust the memory.

The rate you wish to send data might be much higher than the rate the data channel can
actually send to the peer over the Internet. The above properties/methods help your
application to pace the amount of data to be pushed into the data channel.


## How to run the example code

This is a CLI application that demonstrates WebRTC data channel flow control between two separate instances/machines. Users manually exchange SDP offers and answers to establish peer-to-peer connections.

```
                        manual SDP exchange
           +----------------------------------------+
           |                                        |
           v                                        v
   +---------------+                        +---------------+
   |               |          data          |               |
   |     sender    |----------------------->|    receiver   |
   |:PeerConnection|                        |:PeerConnection|
   +---------------+                        +---------------+
```

### Usage

At the root of the example, `pion/webrtc/examples/data-channels-flow-control/`:

```bash
# Show help
$ go run *.go

# Start sender (creates offer)
$ go run *.go send

# Start receiver (responds to offer) 
$ go run *.go receive
```

### Instructions

1. **Start sender**: Run `go run *.go send` on one machine/terminal
2. **Start receiver**: Run `go run *.go receive` on another machine/terminal  
3. **Exchange SDPs**: Copy the SDP output from sender and paste into receiver
4. **Complete handshake**: Copy the SDP answer from receiver and paste into sender
5. **Data transfer**: The data channel will establish and start transferring data

Once the data channel is successfully opened, the sender will start sending a series of 1024-byte packets to the receiver as fast as it can while respecting buffer limits, until you kill the process with Ctrl-C.

### Example Output

**Sender:**
```
2019/08/31 14:56:41 OnOpen: data-824635025728. Start sending a series of 1024-byte packets as fast as it can
```

**Receiver:**
```
2019/08/31 14:56:41 OnOpen: data-824637171120. Start receiving data
2019/08/31 14:56:42 Throughput: 179.118 Mbps
2019/08/31 14:56:43 Throughput: 203.545 Mbps
2019/08/31 14:56:44 Throughput: 211.516 Mbps
2019/08/31 14:56:45 Throughput: 216.292 Mbps
2019/08/31 14:56:46 Throughput: 217.961 Mbps
2019/08/31 14:56:47 Throughput: 218.342 Mbps
 :
```
