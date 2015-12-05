package connection

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/runeaune/bitcoin-network/messages"
)

var MainNetStartString = []byte{0xf9, 0xbe, 0xb4, 0xd9}

// TODO add support for test nets.
var TestNetStartString = []byte{0x0b, 0x11, 0x09, 0x07}
var RegTestStartString = []byte{0xfa, 0xbf, 0xb5, 0xda}

// Send ping if no activity for this many seconds.
const kInactivitySeconds = 120

// Disconnect if no activity for this many sencond.
const kTimeoutSeconds = 180

// Abort connection attempt if no response within this many seconds.
const kDialTimeoutSeconds = 10

// Maximum pending outgoing messages before messages start getting dropped.
const kSendBufferSize = 10

type Conn interface {
	// Name/address of the endpoint we're connected to.
	Endpoint() string

	// Write to this channel to send messages on the connection.
	WriteChannel() chan<- Message

	// Connection failed to set up and should not be used or closed. Only
	// initial errors will set this, later errors will come through
	// the channel provided to Run().
	Error() error

	// Start the operation of a connected connection. Errors and received
	// messages will be passed on the provided channel. The caller can
	// safely close the channel after Close() returns.
	Run(chan<- Message)

	// Close connection. Blocks until it's done, specifically it's safe to
	// close the channel provided to Run() when this call returns. Only
	// call when connected, ie. when Error() == nil.
	Close()

	MostRecentActivity() time.Time
}

type Config struct {
	Version          int32
	MinRemoteVersion int32
	Services         uint64
	UserAgent        string
	StartHeight      int32
	Relay            bool
}

// Send will add a message to the connection's WriteChannel, or fail if the
// channel's buffer is full.
func Send(c Conn, m Message) error {
	select {
	case c.WriteChannel() <- m:
		// do nothing.
	default:
		return fmt.Errorf("Send queue to %q full; dropping %q message.",
			c.Endpoint(), m.Type)
	}
	return nil
}

type ConnImpl struct {
	conn        net.Conn
	err         error
	endpoint    string
	localConfig Config

	mostRecentActivity time.Time // time of last receive.
	sendCh             chan Message
	readCh             chan<- Message

	quit chan bool
	done chan struct{}
}

// Connect connectes to the given endpoint and returns a Connection when the
// connection has been established (or failed -- check Error()). If the
// connection was successful, operations may be started by calling Run(), and
// must be stopped with Close().
func Connect(endpoint string, config Config) Conn {
	c := &ConnImpl{
		endpoint:    endpoint,
		sendCh:      make(chan Message, kSendBufferSize),
		localConfig: config,
	}
	conn, err := net.DialTimeout("tcp", c.endpoint,
		kDialTimeoutSeconds*time.Second)
	if err != nil {
		c.err = err
		return c
	}
	c.conn = conn
	c.mostRecentActivity = time.Now()

	err = sendMessage(c.conn, "version",
		versionMessage(c.localConfig).Generate())
	if err != nil {
		c.conn.Close()
		c.err = err
		return c
	}
	return c
}

func (c *ConnImpl) Close() {
	c.err = fmt.Errorf("Disconnected.")

	// Signal to shut down the internal thread if started.
	if c.quit != nil {
		c.quit <- true
	}

	// Closing conn will cause the input thread to
	// stop immediately, but apparently large
	// writes may hang for quite a while, so the
	// output thread will still be running.
	if c.conn != nil {
		c.conn.Close()
	}

	// Wait until no more communication can occur. Internal operations may
	// continue for a while before geting garbage collected, but for all
	// practical purposes the connection will be closed.
	if c.done != nil {
		<-c.done
	}
}

func (c *ConnImpl) WriteChannel() chan<- Message {
	return c.sendCh
}
func (c *ConnImpl) Endpoint() string {
	return c.endpoint
}

func (c *ConnImpl) Error() error {
	return c.err
}

func (c *ConnImpl) MostRecentActivity() time.Time {
	return c.mostRecentActivity
}

// startMainThread starts the main loop than handles incoming and outgoing
// messages. Incoming messages are forwarded to the read channel provided.
func (c *ConnImpl) Run(readCh chan<- Message) {
	if c.conn == nil {
		log.Fatalf("Not connected.")
	}
	if readCh == nil {
		log.Fatalf("Read channel is nil.")
	}
	c.readCh = readCh

	// Flag and channels used to coordinate shutting down.
	var closing bool // true when closing; don't send anything anywhere.
	c.quit = make(chan bool)
	c.done = make(chan struct{})

	// We're using a local copy of the send channel to block acceptance of
	// messages while busy sending the last. The channel is buffered so
	// under normal circumstances this won't cause the sender to block.
	sendCh := c.sendCh

	// Start threads handling the actual receiving and sending.
	input := startInputThread(c.endpoint, c.conn)
	output, result := startOutputThread(c.conn)

	maintenanceTicker := time.NewTicker(10 * time.Second)
	go func() {
		// Loop until all internal channels are closed. The connection
		// will close before this, but we need to make sure we drain
		// everything.
		for input != nil || result != nil {
			select {
			case <-c.quit:
				closing = true

				close(output)

				maintenanceTicker.Stop()

				// Although some internal process are still
				// alive, there will be no more communication
				// from us so it's safe to let Close() return.
				close(c.done)

			case msg, ok := <-sendCh:
				// Don't accept another message until the send
				// completes.
				sendCh = nil
				if !ok {
					c.sendCh = nil
					continue
				}
				output <- msg

			case err, ok := <-result:
				if !ok || closing {
					result = nil
					continue
				}
				c.errorReporter(err)

				// We're ready for another message.
				sendCh = c.sendCh

			case msg, ok := <-input:
				if !ok || closing {
					input = nil
					continue
				}
				if msg.Error() != nil {
					c.readCh <- msg
					continue
				}
				c.handleReceivedMessage(msg)

			case <-maintenanceTicker.C:
				if closing {
					continue
				}
				c.handleMaintenanceTick(output)
			}
		}
	}()
}

// The input thread will read messages from the connection, parse them and send
// them of to the main loop.
func startInputThread(name string, conn net.Conn) <-chan Message {
	ch := make(chan Message)

	go func() {
		defer func() { close(ch) }()
		for {
			command, data, err := ParseOneMessage(conn, MainNetStartString)
			if err != nil {
				// Report an error even if the failure is EOF (conn closed),
				// which is not trivial to separate from other errors. If
				// conn is closed, the closing flag in the main loop will
				// stop the error from propagating.
				ch <- ErrorMessage(name,
					fmt.Errorf("Receive from %q failed: %v.", name, err))
				return
			}
			ch <- Message{
				Endpoint: name,
				Type:     command,
				Data:     data,
			}
		}
	}()
	return ch
}

// The output thread will receive messages from the main loop, send them on the
// connection and return the result. The rationale is to avoid blocking the
// main thread during large sends, allowing it to continue operating, or shut
// down cleanly without having to wait for the send to complete.
func startOutputThread(conn net.Conn) (chan<- Message, <-chan error) {
	output := make(chan Message, kSendBufferSize)
	result := make(chan error)

	go func() {
		defer func() { close(result) }()
		for {
			m, ok := <-output
			if !ok {
				return
			}
			result <- sendMessage(conn, m.Type, m.Data)
		}
	}()
	return output, result
}

// errorReporter send an error on the error channel,
// unless the error is nil.
func (c ConnImpl) errorReporter(err error) {
	if err != nil {
		c.readCh <- ErrorMessage(c.Endpoint(), err)
	}
}

// handleReceivedMessage handles maintenance messages and forwards all other messages
// on the receive channel.
func (c *ConnImpl) handleReceivedMessage(msg Message) {
	c.mostRecentActivity = time.Now()
	switch msg.Type {
	case "ping":
		nonce := messages.PingPongExtractNonce(msg.Data)
		err := sendMessage(c.conn, "pong",
			messages.PingPongInsertNonce(nonce))
		c.errorReporter(err)
	case "pong":
		// Ignore pong except for updating the activity timer.
	case "verack":
		// Ignore verack except for updating the activity timer.
	case "version":
		v, err := messages.ParseVersionMessage(msg.Data)
		if err != nil {
			c.errorReporter(fmt.Errorf("Failed to parse version message: %v",
				msg.Endpoint, err))
		} else if v.Version < c.localConfig.MinRemoteVersion {
			c.errorReporter(fmt.Errorf("Remote peer is running old version (%d).",
				v.Version))
		} else {
			// The version message is propagated so that other layers are
			// made aware of the connection and what it offers.
			c.readCh <- msg
		}
	default:
		c.readCh <- msg
	}
}

// handleMaintenanceTick will run at periodic interval check the state of the connection.
// If nothing has been received for kInactivitySeconds, a ping will be sent. If nothing has
// been received for kTimeoutSeconds (implying no response to the ping), an error is reported.
func (c *ConnImpl) handleMaintenanceTick(output chan<- Message) {
	idle := time.Since(c.mostRecentActivity)
	if idle > kTimeoutSeconds*time.Second {
		// Tell users of the connection that it has timed out.
		// This will typically result in the connection being shut down.
		c.errorReporter(fmt.Errorf("Connection timed out."))
	} else if idle > kInactivitySeconds*time.Second {
		log.Printf("No activity from %q; sending ping.", c.endpoint)
		output <- Message{
			Type: "ping",
			Data: messages.PingPongInsertRandomNonce(),
		}
	}
}
