//go:build windows

package device

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Protect encrypts data for the current user (DPAPI).
func Protect(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	in := windows.DataBlob{
		Size: uint32(len(plaintext)),
		Data: &plaintext[0],
	}
	var out windows.DataBlob
	err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	cp := make([]byte, out.Size)
	copy(cp, unsafe.Slice(out.Data, out.Size))
	return cp, nil
}

// Unprotect decrypts DPAPI blob for current user.
func Unprotect(blob []byte) ([]byte, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("empty blob")
	}
	in := windows.DataBlob{
		Size: uint32(len(blob)),
		Data: &blob[0],
	}
	var out windows.DataBlob
	err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data))) }()
	cp := make([]byte, out.Size)
	copy(cp, unsafe.Slice(out.Data, out.Size))
	return cp, nil
}

// ProtectAvailable reports DPAPI available.
func ProtectAvailable() bool { return true }
