package coecrypto

import (
	"crypto/cipher"
	"errors"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// NonceFromSeq builds the 12-byte AEAD nonce: 4 zero bytes || seq u64 BE.
func NonceFromSeq(seq uint64) []byte {
	return Concat(I2OSP(0, 4), I2OSP(seq, 8))
}

// BuildAAD constructs DATA AAD per spec.
func BuildAAD(ver uint8, epochID uint64, senderID, receiverID string, seq uint64) []byte {
	return Concat(
		[]byte("COE-v1-data"),
		[]byte{ver},
		I2OSP(epochID, 8),
		[]byte(senderID),
		[]byte(receiverID),
		I2OSP(seq, 8),
	)
}

// Seal encrypts plaintext with ChaCha20-Poly1305. Returns ciphertext||tag.
func Seal(key, nonce, aad, plaintext []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("aead key must be %d bytes", chacha20poly1305.KeySize)
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("nonce must be %d bytes", aead.NonceSize())
	}
	return aead.Seal(nil, nonce, plaintext, aad), nil
}

// Open decrypts ciphertext||tag.
func Open(key, nonce, aad, ciphertext []byte) ([]byte, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("aead key must be %d bytes", chacha20poly1305.KeySize)
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("nonce must be %d bytes", aead.NonceSize())
	}
	pt, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errors.New("aead open failed")
	}
	return pt, nil
}

// NewAEAD returns a cipher.AEAD for advanced use.
func NewAEAD(key []byte) (cipher.AEAD, error) {
	return chacha20poly1305.New(key)
}
