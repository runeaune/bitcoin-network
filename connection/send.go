package connection

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/runeaune/bitcoin-network/messages"
)

func checksum(data []byte) []byte {
	first := sha256.Sum256(data)
	sum := sha256.Sum256(first[:])
	return sum[0:4]
}

// Package and send a message.
func sendMessage(conn net.Conn, cmd string, data []byte) error {
	packed := packageMessage(cmd, data)
	n, err := conn.Write(packed)
	if err != nil || n != len(packed) {
		return fmt.Errorf("Failed to send message: %v.", err)
	}
	return nil
}

// TODO Add support for testnet.
func packageMessage(cmd string, data []byte) []byte {
	var b bytes.Buffer

	b.Write(MainNetStartString)

	// Command name
	field := make([]byte, 12)
	copy(field, []byte(cmd))
	b.Write(field)

	binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(checksum(data))
	b.Write(data)

	return b.Bytes()
}

func versionMessage(c Config) *messages.VersionMessage {
	m := messages.NewVersionMessage()
	m.Version = c.Version
	m.Services = c.Services
	m.UserAgent = c.UserAgent
	m.StartHeight = c.StartHeight
	m.Relay = c.Relay
	return m
}
