package messages_test

import (
	"bytes"

	"testing"

	"github.com/runeaune/bitcoin-network/messages"
)

func TestWriteCompactUint(t *testing.T) {
	var err error
	var r uint
	b := make([]byte, 0, 16)
	buf := bytes.NewBuffer(b)

	val10 := []byte{0x0a}
	err = messages.WriteCompactUint(10, buf)
	if err != nil {
		t.Fatalf("Failed to write value: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), val10) {
		t.Errorf("Failed to write 10, got %x, expected %x", buf, val10)
	}
	r, err = messages.ParseCompactUint(buf)
	if err != nil || r != 10 {
		t.Fatalf("Failed to read back value, got: %d, %v", r, err)
	}

	buf.Reset()

	val1000 := []byte{0xfd, 0xe8, 0x03}
	err = messages.WriteCompactUint(1000, buf)
	if err != nil {
		t.Fatalf("Failed to write value: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), val1000) {
		t.Errorf("Failed to write 1000, got %x, expected %x", buf, val1000)
	}
	r, err = messages.ParseCompactUint(buf)
	if err != nil || r != 1000 {
		t.Fatalf("Failed to read back value, got: %d, %v", r, err)
	}
	buf.Reset()

	val1e6 := []byte{0xfe, 0x40, 0x42, 0x0f, 0x00}
	err = messages.WriteCompactUint(1e6, buf)
	if err != nil {
		t.Fatalf("Failed to write value: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), val1e6) {
		t.Errorf("Failed to write 1e6, got %x, expected %x", buf, val1e6)
	}
	r, err = messages.ParseCompactUint(buf)
	if err != nil || r != 1e6 {
		t.Fatalf("Failed to read back value, got: %d, %v", r, err)
	}
	buf.Reset()

	val1e10 := []byte{0xff, 0x00, 0xe4, 0x0b, 0x54, 0x02, 0x00, 0x00, 0x00}
	err = messages.WriteCompactUint(1e10, buf)
	if err != nil {
		t.Fatalf("Failed to write value: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), val1e10) {
		t.Errorf("Failed to write 1e10, got %x, expected %x", buf, val1e10)
	}
	r, err = messages.ParseCompactUint(buf)
	if err != nil || r != 1e10 {
		t.Fatalf("Failed to read back value, got: %d, %v", r, err)
	}
	buf.Reset()
}
