package messages

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Hash []byte

func (h Hash) String() string {
	reverse := make([]byte, len(h))
	for i, c := range h {
		reverse[len(h)-i-1] = c
	}
	return fmt.Sprintf("%x", reverse)
}

func ParseCompactUint(r io.Reader) (uint, error) {
	b := make([]byte, 1)
	n, err := r.Read(b)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("Failed to read initial byte: %v", err)
	}
	c := uint(b[0])
	if c <= 0xfc {
		return c, nil
	} else if c == 0xfd {
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint(v), err
	} else if c == 0xfe {
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint(v), err
	} else if c == 0xff {
		var v uint64
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint(v), err
	}
	return 0, fmt.Errorf("Initial byte has unrecognized value %d.", c)
}

func WriteCompactUint(val uint, w io.Writer) error {
	var v interface{}
	var err error
	if val <= 0xfc {
		v = uint8(val)
	} else if val <= math.MaxUint16 {
		b := []byte{0xfd}
		_, err = w.Write(b)
		v = uint16(val)
	} else if val <= math.MaxUint32 {
		b := []byte{0xfe}
		_, err = w.Write(b)
		v = uint32(val)
	} else {
		b := []byte{0xff}
		_, err = w.Write(b)
		v = uint64(val)
	}
	if err != nil {
		return fmt.Errorf("Failed to write length indicator for val %v: %v",
			val, err)
	}
	err = binary.Write(w, binary.LittleEndian, v)
	if err != nil {
		return fmt.Errorf("Failed to write value %v: %v", val, err)
	}
	return nil
}

func ParseVarBytes(r io.Reader) ([]byte, error) {
	l, err := ParseCompactUint(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to get length: %v", err)
	}
	b := make([]byte, l)
	n, err := r.Read(b)
	if err != nil {
		return nil, fmt.Errorf("Failed to read string: %v", err)
	}
	if n != int(l) {
		return nil, fmt.Errorf("Length mismatch: Expected %d, got %d", l, n)
	}
	return b, nil
}

func ParseVarString(r io.Reader) (string, error) {
	b, err := ParseVarBytes(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func WriteVarString(s string, w io.Writer) error {
	err := WriteCompactUint(uint(len(s)), w)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(s))
	if err != nil {
		return err
	}
	return nil
}

func ParseBytes(r io.Reader, l int) ([]byte, error) {
	b := make([]byte, l)
	n, err := r.Read(b)
	if err != nil {
		return nil, fmt.Errorf("Failed to read bytefield: %v", err)
	}
	if n != l {
		return nil, fmt.Errorf("Length mismatch: Expected %d, got %d", l, n)
	}
	return b, nil
}
