// The network layer maintains a list of peer nodes and connections to some of
// them. getaddr messages are sent to connected nodes in order to improve the
// view of the network. Broken connections are automatically replaced.

package network

import (
	"log"
	"sort"

	"github.com/aarbt/bitcoin-network/connection"
	"github.com/aarbt/bitcoin-network/messages"
)

const kMainNetDefaultPort = 8333
const kMinPeerVersion = 70001

// Package of data going to or comming from a connected peer.
type Message struct {
	Endpoint string
	Type     string
	Data     []byte
}

// Config passed when creating a new Network.
type Config struct {
	// Number of peers the network should aim to connect to.
	DesiredConnections int
	// Path to file used to storing good peers for loading on next startup.
	PeerStorageFile string
	// List of hostnames that will be looked up in DNS to get IPs of peers.
	SeedHostnames []string
	// Channel to send all received messages that are not handled at the
	// connection or network layer. Will not be written to after Close()
	// returns.
	OutputChannel chan<- Message
}

// Network sets up a communication network with peers fed to it from DNS, loaded from
// its storage, and shared from existing peers. The aim is to get a robust network
// that will give the creator a good overview of the complete Bitcoin network, even if
// some of the peers are bad actors.
type Network struct {
	config      Config
	peers       map[string]*peer
	newPeerCh   chan PeerAddress
	connectedCh chan connection.Conn

	sendCh    chan Message
	receiveCh chan connection.Message

	done    chan struct{} // Closes when shut down is complete.
	closing bool
}

func New(config Config) *Network {
	n := Network{
		config: config,
		peers:  make(map[string]*peer),
		// Buffer peer channel to reduce number of expensive
		// maybeConnectToMorePeers runs when large batches of peers are
		// added. This also reduces the risk of a peer reporter hanging
		// due to the main netowrk loop having exited.
		newPeerCh: make(chan PeerAddress, 100),

		// Make enough room for the maximum number of pending
		// connections to avoid blocking pending connections if Network
		// was shut down while a connection was set up.
		connectedCh: make(chan connection.Conn, config.DesiredConnections),

		sendCh:    make(chan Message),
		receiveCh: make(chan connection.Message),

		done: make(chan struct{}),
	}
	n.startMainThread()

	go loadPeers(n.config.PeerStorageFile, n.newPeerCh)
	for _, name := range n.config.SeedHostnames {
		go n.addPeersFromDNSLookup(name)
	}
	return &n
}

// Close will shut down the whole network cleanly, with the caveat that it's unsafe to
// call any further methods on the network object while or after Close() has been called.
func (n *Network) Close() {
	close(n.sendCh)
	<-n.done
}

// close is an internal function that will be called from the main loop when sendCh
// gets closed.
func (n *Network) close() {
	if err := storePeers(n.config.PeerStorageFile, n.peers); err != nil {
		log.Printf("Failed to store peers: %v", err)
	}

	var open int
	closed := make(chan int)
	for _, p := range n.peers {
		if p.isConnected() {
			open++
			go func(p *peer) {
				log.Printf("Closing connection to %q.", p.name())
				p.close()
				closed <- 1
			}(p)
		}
	}
	go func() {
		for open > 0 {
			<-closed
			open--
		}
		// Once all connected peers are shut down it's safe to close the
		// receive channel.
		close(n.receiveCh)
	}()
}

func (n *Network) maybeConnectToMorePeers() {
	if n.closing {
		return
	}
	count := 0
	list := make([]*peer, 0, len(n.peers))
	for _, p := range n.peers {
		list = append(list, p)
		if p.isConnected() || p.isPending() {
			count++
		}
	}
	sort.Sort(PeersByQuality(list))
	for _, p := range list {
		if p.isConnected() || p.isPending() {
			continue
		}
		if p.Quality() < 0 {
			// Remaining entries in the list are banned peers.
			return
		}
		if count >= n.config.DesiredConnections {
			return
		}
		count++
		p.connect(connection.Connect, n.connectedCh)
	}
}

func (n *Network) SendChannel() chan<- Message {
	return n.sendCh
}

func (n *Network) startMainThread() {
	go func() {
		// Run the main loop until send and receive channel has been closed.
		for n.sendCh != nil || n.receiveCh != nil {
			select {
			case peer := <-n.newPeerCh:
				n.readAndHandlePeerAddresses(peer)

			case c := <-n.connectedCh:
				if n.closing {
					continue
				}
				p, found := n.peers[c.Endpoint()]
				if !found {
					log.Fatalf("Connection to invalid peer %q.",
						c.Endpoint())
				}
				if err := p.connected(c); err != nil {
					p.setError(err)
					log.Println(err)
					n.maybeConnectToMorePeers()
				}

			case msg, ok := <-n.sendCh:
				if !ok {
					n.sendCh = nil
					// Avoiding more connections getting
					// opened and throw away those pending.
					n.closing = true
					n.close()
					continue
				}
				n.handleSend(msg)

			case msg, ok := <-n.receiveCh:
				if !ok {
					n.receiveCh = nil
					continue
				}
				if msg.Error() != nil {
					n.handleError(msg.Endpoint, msg.Error())
					continue
				}
				n.handleReceivedMessage(msg)
			}
		}
		close(n.done)
	}()
}

func (n *Network) handleError(source string, e error) {
	peer, err := findConnectedPeer(n.peers, source)
	if err != nil {
		log.Printf("Error (%v) from unknown peer: %v", e, err)
	} else {
		log.Printf("Error (%v); closing connection.", e)
		peer.setError(e)
		peer.close()
		n.maybeConnectToMorePeers() // consider opening more connections.
	}
}

func (n *Network) handleSend(msg Message) {
	connMsg := connection.NewMessage(msg.Endpoint, msg.Type, msg.Data)
	if msg.Endpoint == "" {
		// Broadcast message to all connected peers.
		for _, p := range n.peers {
			if p.isConnected() {
				msg.Endpoint = p.name()
				// Send is non-blocking. It might fail if the
				// connection's send buffer is full, but we don't
				// care about that for broadcasts.
				_ = connection.Send(p.conn, connMsg)
			}
		}
	} else {
		p, err := findConnectedPeer(n.peers, msg.Endpoint)
		if err != nil {
			log.Printf("Can't send message: %v", err)
		} else {
			if err := connection.Send(p.conn, connMsg); err != nil {
				log.Println(err)
			}
		}
	}
}

func (n *Network) handleReceivedMessage(msg connection.Message) {
	switch msg.Type {
	case "addr":
		addrs, err := messages.ParseAddrVector(msg.Data)
		if err != nil {
			log.Printf("Failed to parse address vector from %q: %v",
				msg.Endpoint, err)
			return
		}
		for _, a := range addrs {
			n.handlePeerAddress(PeerAddress{
				Address:  a.Key(),
				Time:     a.Time,
				Services: a.Services,
				Reporter: msg.Endpoint,
			})
		}
		n.maybeConnectToMorePeers() // connect to new peers as needed.

	default:
		if n.config.OutputChannel != nil {
			n.config.OutputChannel <- Message{
				Endpoint: msg.Endpoint,
				Type:     msg.Type,
				Data:     msg.Data,
			}
		}
	}
}

// readAndHandlePeerAddresses will add the passed address, plus any found pending
// in the peer channel, to the peer list, then call maybeConnectToMorePeers() once.
// readAndHandlePeerAddresses should only be called from the main loop to avoid races.
func (n *Network) readAndHandlePeerAddresses(addr PeerAddress) {
	addrs := []PeerAddress{addr}
	// Empty out the channel for pending peers and add them to our list.
	done := false
	for !done {
		select {
		case a := <-n.newPeerCh:
			addrs = append(addrs, a)
		default:
			done = true
		}
	}
	for _, a := range addrs {
		n.handlePeerAddress(a)
	}
	n.maybeConnectToMorePeers() // connect to peers as needed.
}

// handlePeerAddress should only be called from the main thread to avoid races.
func (n *Network) handlePeerAddress(addr PeerAddress) {
	p, found := n.peers[addr.Address]
	if found {
		if addr.Time.After(p.Time) {
			p.Time = addr.Time
		}
		if addr.FailureTime.After(p.FailureTime) {
			p.FailureTime = addr.FailureTime
		}
	} else {
		n.peers[addr.Address] = newPeer(addr, n.receiveCh)
	}
}
