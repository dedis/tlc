package qscas

import (
	"crypto/rand"
	"encoding/binary"
)

// Generate a 63-bit positive integer from strong cryptographic randomness.
func randValue() int64 {
	var b [8]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic("error reading cryptographic randomness: " + err.Error())
	}
	return int64(binary.BigEndian.Uint64(b[:]) &^ (1 << 63))
}
