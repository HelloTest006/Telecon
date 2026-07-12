package device

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// IssueSignPayload is the canonical bytes signed for POST /v1/key/issue.
func IssueSignPayload(deviceID string, nonce []byte, clientTime int64, profile string) []byte {
	// COE-v1-issue || device_id || nonce || client_time_be64 || profile
	buf := make([]byte, 0, 64+len(deviceID)+len(nonce)+len(profile))
	buf = append(buf, []byte("COE-v1-issue")...)
	buf = append(buf, 0)
	buf = append(buf, []byte(deviceID)...)
	buf = append(buf, 0)
	buf = append(buf, nonce...)
	var tb [8]byte
	binary.BigEndian.PutUint64(tb[:], uint64(clientTime))
	buf = append(buf, tb[:]...)
	buf = append(buf, []byte(profile)...)
	return buf
}

// SignIssue signs an issue request with Ed25519.
func SignIssue(signSK ed25519.PrivateKey, deviceID string, nonce []byte, clientTime int64, profile string) []byte {
	return ed25519.Sign(signSK, IssueSignPayload(deviceID, nonce, clientTime, profile))
}

// VerifyIssue checks Ed25519 signature.
func VerifyIssue(signPK ed25519.PublicKey, deviceID string, nonce []byte, clientTime int64, profile, sig []byte) bool {
	if len(signPK) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(signPK, IssueSignPayload(deviceID, nonce, clientTime, string(profile)), sig)
}

// GenerateSignKey creates Ed25519 keypair for KA auth.
func GenerateSignKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// SignSKFromSeed builds private key from 32-byte seed or 64-byte private.
func SignSKFromBytes(b []byte) (ed25519.PrivateKey, error) {
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(b), nil
	default:
		return nil, fmt.Errorf("sign key must be %d or %d bytes", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}
