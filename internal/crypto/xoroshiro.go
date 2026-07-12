package coecrypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"sync"
)

// Xoroshiro128pp is xoroshiro128++ for non-secret tickets only.
type Xoroshiro128pp struct {
	s0, s1 uint64
	mu     sync.Mutex
}

// NewXoroshiroFromMaster seeds from KA_MASTER and a process nonce (CSPRNG).
func NewXoroshiroFromMaster(kaMaster []byte) (*Xoroshiro128pp, error) {
	processNonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, processNonce); err != nil {
		return nil, err
	}
	h := sha256.Sum256(Concat(kaMaster, []byte("coe-xoroshiro-seed"), processNonce))
	s0 := binary.LittleEndian.Uint64(h[0:8])
	s1 := binary.LittleEndian.Uint64(h[8:16])
	if s0 == 0 && s1 == 0 {
		s1 = 1
	}
	return &Xoroshiro128pp{s0: s0, s1: s1}, nil
}

func rotl(x uint64, k int) uint64 {
	return (x << k) | (x >> (64 - k))
}

// NextU64 advances and returns next uint64.
func (x *Xoroshiro128pp) NextU64() uint64 {
	x.mu.Lock()
	defer x.mu.Unlock()
	s0, s1 := x.s0, x.s1
	result := rotl(s0+s1, 17) + s0
	s1 ^= s0
	x.s0 = rotl(s0, 49) ^ s1 ^ (s1 << 21)
	x.s1 = rotl(s1, 28)
	return result
}

// RequestID returns 16-byte public ticket from PRNG stream.
func (x *Xoroshiro128pp) RequestID() []byte {
	b := make([]byte, 16)
	binary.BigEndian.PutUint64(b[0:8], x.NextU64())
	binary.BigEndian.PutUint64(b[8:16], x.NextU64())
	return b
}

// PublicTag returns 8-byte public tag.
func (x *Xoroshiro128pp) PublicTag() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, x.NextU64())
	return b
}
