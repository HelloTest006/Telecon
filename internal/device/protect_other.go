//go:build !windows

package device

import "fmt"

// Protect is a no-op encrypt on non-Windows (still stores; mark clearly).
// Production non-Windows should use keyring/libsecret — not implemented here.
func Protect(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	// prefix so Load can detect unprotected
	out := make([]byte, 0, 4+len(plaintext))
	out = append(out, []byte("RAW1")...)
	out = append(out, plaintext...)
	return out, nil
}

// Unprotect reverses Protect on non-Windows.
func Unprotect(blob []byte) ([]byte, error) {
	if len(blob) >= 4 && string(blob[:4]) == "RAW1" {
		return append([]byte(nil), blob[4:]...), nil
	}
	// legacy plaintext identity files
	return append([]byte(nil), blob...), nil
}

// ProtectAvailable is false off Windows (only opaque wrapper).
func ProtectAvailable() bool { return false }
