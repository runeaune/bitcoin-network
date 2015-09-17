// Methods used when initializing the network during start-up. Previously
// known peers are loaded from storage and new ones are fetched from DNS.

package network

import (
	"fmt"
	"net"
	"time"
)

// addPeersFromDNSLookup adds the IPs returned from a DNS lookup of the
// provided name as peers.
func (n *Network) addPeersFromDNSLookup(name string) error {
	ips, err := net.LookupIP(name)
	if err != nil {
		return fmt.Errorf("DNS lookup failed: %v", err)
	}
	for _, ip := range ips {
		// Assuming freshness is time.Now() for data from DNS lookup.
		n.reportPeer(ip, kMainNetDefaultPort, time.Now(), name)
	}
	return nil
}

func (n *Network) reportPeer(ip net.IP, port uint16, lastSeen time.Time, reporter string) {
	key := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))
	addr := PeerAddress{
		Address:  key,
		Time:     lastSeen,
		Reporter: reporter,
	}

	// Stuff the address onto the peer channel to be handled by handlePeerAddress
	// called from the main loop.
	n.newPeerCh <- addr
}
