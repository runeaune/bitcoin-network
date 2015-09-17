// Functions for storing and loading peer information to a local file.

package network

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

func storePeers(filename string, peers map[string]*peer) error {
	if filename == "" {
		return nil
	}
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %v", filename, err)
	}
	defer file.Close()
	return writePeers(file, peers)
}

func writePeers(dst io.Writer, peers map[string]*peer) error {
	w := bufio.NewWriter(dst)
	for _, p := range peers {
		_, err := w.WriteString(p.marshal())
		if err != nil {
			return fmt.Errorf("Failed to write %q: %v", p.marshal(), err)
		}
	}
	w.Flush()
	return nil
}

func loadPeers(filename string, ch chan<- PeerAddress) {
	if filename == "" {
		return
	}
	file, err := os.Open(filename)
	if err != nil {
		log.Printf("Failed to load peers: %v", err)
		return
	}
	defer file.Close()

	peers, err := readPeers(file)
	if err != nil {
		log.Printf("Failed to load peers: %v", err)
	} else {
		for _, addr := range peers {
			ch <- addr
		}
	}
}

func readPeers(src io.Reader) ([]PeerAddress, error) {
	var peers []PeerAddress
	s := bufio.NewScanner(src)
	for s.Scan() {
		addr, err := unmarshalPeerAddress(s.Text())
		if err != nil {
			log.Println(err)
			continue
		}
		peers = append(peers, addr)
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("Failed to read from file: %v", err)
	}
	return peers, nil
}

func unixToTime(unix int64, def time.Time) time.Time {
	if unix != 0 {
		return time.Unix(unix, 0)
	} else {
		return def
	}
}

func unmarshalPeerAddress(data string) (PeerAddress, error) {
	var host string
	var activityTime, failureTime int64
	_, err := fmt.Sscanf(data, "%s %d %d",
		&host, &activityTime, &failureTime)
	if err != nil {
		return PeerAddress{}, fmt.Errorf("Couldn't unmarshal %q: %v",
			data, err)
	}
	return PeerAddress{
		Address:     host,
		Time:        unixToTime(activityTime, time.Now()),
		FailureTime: unixToTime(failureTime, time.Time{}),
	}, nil
}
