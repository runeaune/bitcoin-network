// Data structures and methods relating to a single peer. The peer object is
// intended for local use through a Network object only. With the exception of
// the read-only methods MostRecentActivity(), Quality() and
// String(), methods should not be called concurrently.

package network

import (
	"fmt"
	"log"
	"time"

	"github.com/aarbt/bitcoin-network/connection"
)

const kPeerGoodQuality = 500

type PeerAddress struct {
	Address     string
	Reporter    string
	Services    uint64
	Time        time.Time
	FailureTime time.Time
}

func (a PeerAddress) String() string {
	s := fmt.Sprintf("%q seen by %q at %s", a.Address, a.Reporter, a.Time)
	if !a.FailureTime.IsZero() {
		s += fmt.Sprintf(", failed at %s", a.FailureTime)
	}
	return s
}

type peer struct {
	PeerAddress
	conn      connection.Conn
	BanTime   time.Time
	lastError error
	receiveCh chan<- connection.Message
	pending   bool
}

func newPeer(addr PeerAddress, ch chan<- connection.Message) *peer {
	return &peer{
		PeerAddress: addr,
		receiveCh:   ch,
	}
}

func (p peer) name() string {
	return p.Address
}

func (p *peer) marshal() string {
	return fmt.Sprintf("%s %d %d\n", p.Address,
		p.MostRecentActivity().Unix(),
		p.FailureTime.Unix())
}

func (p *peer) setError(err error) {
	if err != nil {
		p.FailureTime = time.Now()
	}
	p.lastError = err
}

func (p *peer) isConnected() bool {
	return p.conn != nil
}

func (p *peer) isPending() bool {
	return p.pending
}

// close shut downs an active connection. Should not be called on unconnected peers.
func (p *peer) close() {
	p.Time = p.MostRecentActivity()

	// Closing the connection will make it close its read channel and cause the
	// peer loop to return.
	p.conn.Close()
	p.conn = nil
}

type connector func(string, connection.Config) connection.Conn

// connect will initate a connection to the peer. The resulting connection
// will be send on the provided channel.
func (p *peer) connect(connector connector, ch chan<- connection.Conn) {
	log.Printf("Connecting to %q.", p.name())
	p.pending = true
	go func() {
		ch <- connector(p.name(), connection.Config{
			Version:          70001,
			Services:         0,
			UserAgent:        "",
			MinRemoteVersion: kMinPeerVersion,
			Relay:            false,
		})
	}()
}

// connected will associate an established connection with the peer.
func (p *peer) connected(c connection.Conn) error {
	if c.Endpoint() != p.name() {
		log.Fatalf("Attempted to associate connection to %q with peer %q.",
			c.Endpoint(), p.name())
	}
	p.pending = false
	if err := c.Error(); err != nil {
		p.FailureTime = time.Now()
		p.lastError = err
		return err
	}
	p.conn = c
	c.Run(p.receiveCh)

	// Ask the peer what other addresses in the network it knows about.
	connection.Send(c, connection.Message{
		Endpoint: p.name(),
		Type:     "getaddr",
	})
	return nil
}

func (p peer) MostRecentActivity() time.Time {
	if p.conn != nil && !p.conn.MostRecentActivity().IsZero() {
		return p.conn.MostRecentActivity()
	} else {
		return p.Time
	}
	return p.Time
}

// Quality returns a quality score for a peer, between -1000 and +1000.
// A high score (>500) indicates an active or recently active peer,
// a lower score indicates inactivity or recent failures, and a negative
// score indicates a banned connection.
func (p peer) Quality() int {
	var score int
	if p.isConnected() {
		score += 200
	}
	active := 800 - int(time.Since(p.MostRecentActivity()).Minutes())
	if active > 0 {
		score += active
	}
	if !p.FailureTime.IsZero() {
		failure := 500 - int(time.Since(p.FailureTime).Minutes())
		if failure > 0 {
			score -= failure
			if score < 0 {
				score = 0
			}
		}
	}
	if !p.BanTime.IsZero() && time.Since(p.BanTime).Seconds() < 0 {
		score = -1000
	}
	return score
}

type PeersByQuality []*peer

func (slice PeersByQuality) Len() int {
	return len(slice)
}

func (slice PeersByQuality) Less(i, j int) bool {
	return slice[i].Quality() > slice[j].Quality()
}

func (slice PeersByQuality) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

// String will return a string describing the peer. It is ok to call it from
// other threads as it's read only, and since it's for human consumption, we
// don't care if it's slightly outdated.
func (p peer) String() string {
	var str string
	if p.isConnected() {
		str = "(connected) "
	} else if p.isPending() {
		str = "(pending)   "
	} else {
		str = "(spare)     "
	}
	str += fmt.Sprintf("%s   %s", p.name(), p.MostRecentActivity())
	if !p.FailureTime.IsZero() {
		str = fmt.Sprintf("(failed)    %s, failed at %s",
			p.name(), p.FailureTime)
	}
	if !p.BanTime.IsZero() {
		str = fmt.Sprintf("(banned)    %s, banned at %s",
			p.name(), p.BanTime)
	}
	str += fmt.Sprintf(" Q(%d)", p.Quality())
	return str
}
