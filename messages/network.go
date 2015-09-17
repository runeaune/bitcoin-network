package messages

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type NetworkAddress struct {
	Time     time.Time
	Services uint64
	Addr     net.IP
	Port     uint16
}

func (a NetworkAddress) String() string {
	return fmt.Sprintf("%s:%d seen at %s (%d), offering %x",
		a.Addr, a.Port, a.Time, a.Time.Unix(), a.Services)
}

func (a NetworkAddress) Key() string {
	return net.JoinHostPort(a.Addr.String(), fmt.Sprintf("%d", a.Port))
}

func ParseNetworkAddress(b io.Reader) (*NetworkAddress, error) {
	a := NetworkAddress{}
	var t uint32
	err := binary.Read(b, binary.LittleEndian, &t)
	if err != nil {
		return nil, fmt.Errorf("Could not read time field: %v", err)
	}
	a.Time = time.Unix(int64(t), 0)

	err = binary.Read(b, binary.LittleEndian, &a.Services)
	if err != nil {
		return nil, fmt.Errorf("Could not read services field: %v", err)
	}
	addr := make([]byte, 16)
	n, err := b.Read(addr)
	if err != nil {
		return nil, fmt.Errorf("Could not read addr field: %v", err)
	}
	if n != len(addr) {
		return nil, fmt.Errorf("Addr field has unexpected length %d (expected 16).", n)
	}
	a.Addr = addr
	err = binary.Read(b, binary.BigEndian, &a.Port)
	if err != nil {
		return nil, fmt.Errorf("Could not read port field: %v", err)
	}
	return &a, nil
}

func ParseAddrVector(data []byte) ([]*NetworkAddress, error) {
	b := bytes.NewBuffer(data)
	count, err := ParseCompactUint(b)
	if err != nil {
		return nil, fmt.Errorf("Could not read count field: %v", err)
	}
	vector := make([]*NetworkAddress, count)
	for i := uint(0); i < count; i++ {
		addr, err := ParseNetworkAddress(b)
		if err != nil {
			return nil, fmt.Errorf("Could not parse network address number %d: %v",
				i, err)
		}
		vector[i] = addr
	}
	return vector, nil
}
