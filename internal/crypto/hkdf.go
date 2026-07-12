package coecrypto

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

// HKDF derives L bytes using HKDF-SHA-256.
func HKDF(ikm, salt, info []byte, l int) ([]byte, error) {
	r := hkdf.New(sha256.New, ikm, salt, info)
	out := make([]byte, l)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

// I2OSP encodes x as n-byte big-endian unsigned integer.
func I2OSP(x uint64, n int) []byte {
	b := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		b[i] = byte(x)
		x >>= 8
	}
	return b
}

// Concat joins byte slices.
func Concat(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// ZeroSalt32 is the normative empty salt for send-key HKDF (32 zero bytes).
var ZeroSalt32 = make([]byte, 32)
