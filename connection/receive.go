package connection

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func readOneByte(input io.Reader) (byte, error) {
	buf := make([]byte, 1)
	n, err := input.Read(buf)
	if err != nil || n != 1 {
		return 0x00, err
	}
	return buf[0], nil
}

// Consume input until a start string is found. The start string is also consumed.
func SeekToNextMessage(input io.Reader, magic []byte) error {
	// We're assuming no reuse of any byte in the magic string, so we
	// don't have to worry about consuming bytes of a subsequent match while
	// testing a partial matche.
	index := 0
	for {
		b, err := readOneByte(input)
		if err != nil {
			return err
		}
		if b == magic[index] {
			index++
		} else {
			if index > 0 {
				// Check missed byte against start of magic string.
				if b == magic[0] {
					index = 1
				} else {
					index = 0
				}
			}
		}
		if index == len(magic) {
			return nil
		}
	}
	return nil
}

func readBytes(r io.Reader, n int) ([]byte, error) {
	field := make([]byte, n)
	for count := 0; count < n; {
		i, err := r.Read(field[count:])
		if err != nil {
			return nil, err
		}
		count += i
	}
	return field, nil
}

func ParseOneMessage(input io.Reader, magic []byte) (string, []byte, error) {
	err := SeekToNextMessage(input, magic)
	if err != nil {
		return "", nil, fmt.Errorf("Error while seeking for next message: %v", err)
	}
	field, err := readBytes(input, 12)
	if err != nil {
		return "", nil, fmt.Errorf("Error reading command field: %v", err)
	}
	command := string(bytes.TrimRight(field, "\x00"))

	var size uint32
	err = binary.Read(input, binary.LittleEndian, &size)
	if err != nil {
		return "", nil, fmt.Errorf("Error reading size field: %v", err)
	}
	if size > (32 << 20) {
		return "", nil, fmt.Errorf("Oversized payload, length %d.", size)
	}

	check, err := readBytes(input, 4)
	if err != nil {
		return "", nil, fmt.Errorf("Error reading checksum field: %v", err)
	}
	var buf []byte
	if size > 0 {
		buf, err = readBytes(input, int(size))
		if err != nil {
			return "", nil, fmt.Errorf("Error reading payload of size %d "+
				"for message %q: %v", size, command, err)
		}
	}
	if !bytes.Equal(check, checksum(buf)) {
		return "", nil, fmt.Errorf("Checksum mismatch: %x != %x",
			checksum(buf), check)
	}
	return command, buf, nil
}
