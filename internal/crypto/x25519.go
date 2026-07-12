package coecrypto

import (
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
)

// GenerateX25519 creates a new key pair. Private is 32 bytes, public 32 bytes.
func GenerateX25519() (private, public []byte, err error) {
	private = make([]byte, curve25519.ScalarSize)
	if _, err = io.ReadFull(rand.Reader, private); err != nil {
		return nil, nil, err
	}
	// clamp is handled by X25519
	public, err = curve25519.X25519(private, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return private, public, nil
}

// X25519 computes shared secret from private scalar and peer public key.
func X25519(private, peerPublic []byte) ([]byte, error) {
	if len(private) != curve25519.ScalarSize || len(peerPublic) != curve25519.PointSize {
		return nil, fmt.Errorf("x25519: bad key length")
	}
	return curve25519.X25519(private, peerPublic)
}

// PublicFromPrivate derives public key from private.
func PublicFromPrivate(private []byte) ([]byte, error) {
	if len(private) != curve25519.ScalarSize {
		return nil, fmt.Errorf("x25519: bad private length")
	}
	return curve25519.X25519(private, curve25519.Basepoint)
}
