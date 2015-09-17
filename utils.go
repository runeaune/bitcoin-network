package network

import (
	"fmt"
	"net/http"
	"sort"
)

func findConnectedPeer(peers map[string]*peer, name string) (*peer, error) {
	peer, found := peers[name]
	if found {
		if peer.isConnected() {
			return peer, nil
		} else {
			return nil, fmt.Errorf("Peer %q not connected", name)
		}
	} else {
		return nil, fmt.Errorf("Peer %q unknown", name)
	}
}

func (n *Network) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	list := make([]*peer, 0, len(n.peers))
	for _, p := range n.peers {
		list = append(list, p)
	}
	sort.Sort(PeersByQuality(list))
	for _, p := range list {
		fmt.Fprintf(w, "%s\n", p)
	}
}
