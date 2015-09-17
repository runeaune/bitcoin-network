package messages

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"log"
)

func PingPongExtractNonce(data []byte) uint64 {
	b := bytes.NewBuffer(data)
	var nonce uint64
	binary.Read(b, binary.LittleEndian, &nonce)
	return nonce
}

func PingPongInsertNonce(nonce uint64) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, nonce)
	return b.Bytes()
}

func PingPongInsertRandomNonce() []byte {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatalf("PingPongInsertRandomNonce error:", err)
	}
	return b
}
