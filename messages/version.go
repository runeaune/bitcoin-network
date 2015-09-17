package messages

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type VersionMessage struct {
	Version             int32
	Services            uint64
	Timestamp           uint64
	ReceiverServices    uint64
	ReceiverIP          net.IP
	ReceiverPort        uint16
	TransmitterServices uint64
	TransmitterIP       net.IP
	TransmitterPort     uint16
	Nonce               uint64
	UserAgent           string
	StartHeight         int32
	Relay               bool

	data []byte
}

func NewVersionMessage() *VersionMessage {
	return &VersionMessage{
		Timestamp:       uint64(time.Now().Unix()),
		ReceiverIP:      net.ParseIP("::ffff:127.0.0.1"),
		ReceiverPort:    8333,
		TransmitterIP:   net.ParseIP("::ffff:127.0.0.1"),
		TransmitterPort: 8333,
	}
}

func NewDefaultVersionMessage() *VersionMessage {
	m := NewVersionMessage()
	m.Version = 70001
	return m
}

func (m *VersionMessage) Generate() []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, m.Version)
	binary.Write(&b, binary.LittleEndian, m.Services)
	binary.Write(&b, binary.LittleEndian, m.Timestamp)

	binary.Write(&b, binary.LittleEndian, m.ReceiverServices)
	b.Write(m.ReceiverIP)
	binary.Write(&b, binary.BigEndian, m.ReceiverPort)

	binary.Write(&b, binary.LittleEndian, m.TransmitterServices)
	b.Write(m.TransmitterIP)
	binary.Write(&b, binary.BigEndian, m.TransmitterPort)
	binary.Write(&b, binary.LittleEndian, m.Nonce)
	WriteVarString(m.UserAgent, &b)

	binary.Write(&b, binary.LittleEndian, m.StartHeight)

	var relay byte
	if m.Relay {
		relay = 0x01
	} else {
		relay = 0x00
	}
	binary.Write(&b, binary.LittleEndian, relay)

	m.data = b.Bytes()
	return m.data
}

func (p VersionMessage) String() string {
	var str string
	str += fmt.Sprintf("Version: %d\n", p.Version)
	str += fmt.Sprintf("Services: %x\n", p.Services)
	str += fmt.Sprintf("Timestamp: %d (%s)\n",
		p.Timestamp, time.Unix(int64(p.Timestamp), 0))
	str += fmt.Sprintf("ReceiverServices: %x\n", p.ReceiverServices)
	str += fmt.Sprintf("Receiver: %s port:%d\n",
		p.ReceiverIP.String(), p.ReceiverPort)
	str += fmt.Sprintf("TransmitterServices: %x\n", p.TransmitterServices)
	str += fmt.Sprintf("Transmitter: %s port:%d\n",
		p.TransmitterIP.String(), p.TransmitterPort)
	str += fmt.Sprintf("Nonce: %d\n", p.Nonce)
	str += fmt.Sprintf("UserAgent: %s\n", p.UserAgent)
	str += fmt.Sprintf("StartHeight: %d\n", p.StartHeight)
	str += fmt.Sprintf("Relay: %v\n", p.Relay)
	return str
}

func ParseVersionMessage(data []byte) (*VersionMessage, error) {
	p := VersionMessage{data: data}
	b := bytes.NewBuffer(data)
	var err error
	err = binary.Read(b, binary.LittleEndian, &p.Version)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse version field: %v", err)
	}
	binary.Read(b, binary.LittleEndian, &p.Services)
	binary.Read(b, binary.LittleEndian, &p.Timestamp)
	binary.Read(b, binary.LittleEndian, &p.ReceiverServices)
	ip := make([]byte, 16)
	b.Read(ip)
	p.ReceiverIP = ip
	binary.Read(b, binary.BigEndian, &p.ReceiverPort)
	binary.Read(b, binary.LittleEndian, &p.TransmitterServices)
	b.Read(ip)
	p.TransmitterIP = ip
	binary.Read(b, binary.BigEndian, &p.TransmitterPort)
	binary.Read(b, binary.LittleEndian, &p.Nonce)
	p.UserAgent, err = ParseVarString(b)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse user agent field: %v", err)
	}
	binary.Read(b, binary.LittleEndian, &p.StartHeight)

	// If relay byte is missing, assume 0.
	c, err := b.ReadByte()
	if err == nil && int(c) > 0 {
		p.Relay = true
	}
	return &p, nil
}
